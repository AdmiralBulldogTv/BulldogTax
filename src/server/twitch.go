package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/AdmiralBulldogTv/BulldogTax/src/auth"
	"github.com/AdmiralBulldogTv/BulldogTax/src/global"
	"github.com/AdmiralBulldogTv/BulldogTax/src/mongo"
	"github.com/AdmiralBulldogTv/BulldogTax/src/structures"
	"github.com/AdmiralBulldogTv/BulldogTax/src/utils"
	"github.com/nicklaw5/helix"
	"github.com/sirupsen/logrus"

	"github.com/gofiber/fiber/v2"

	jsoniter "github.com/json-iterator/go"
)

type WebhookCallback struct {
	Challenge    string                                                 `json:"challenge"`
	Subscription helix.EventSubSubscription                             `json:"subscription"`
	Event        helix.EventSubChannelPointsCustomRewardRedemptionEvent `json:"event"`
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func Twitch(gCtx global.Context, app fiber.Router) {
	app.Get("/login", func(c *fiber.Ctx) error {
		api, err := helix.NewClient(&helix.Options{
			ClientID:     gCtx.Config().Twitch.ClientID,
			ClientSecret: gCtx.Config().Twitch.ClientSecret,
			RedirectURI:  gCtx.Config().Twitch.RedirectURI,
		})
		if err != nil {
			logrus.Fatal("failed to make twitch client: ", err)
		}

		csrfToken, err := utils.GenerateRandomString(64)
		if err != nil {
			logrus.Errorf("secure bytes, err=%e", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		if err != nil {
			logrus.Errorf("secure bytes, err=%e", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		authURL := api.GetAuthorizationURL(&helix.AuthorizationURLParams{
			ResponseType: "code",
			Scopes:       []string{"channel:read:redemptions"},
			State:        csrfToken,
		})

		c.Cookie(&fiber.Cookie{
			Name:     "twitch_csrf",
			Value:    csrfToken,
			Domain:   gCtx.Config().Frontend.CookieDomain,
			Secure:   gCtx.Config().Frontend.CookieSecure,
			HTTPOnly: true,
		})

		return c.Redirect(authURL)
	})

	app.Get("/callback", func(c *fiber.Ctx) error {
		tkn, err := auth.GetAuth(gCtx, c.Context())
		if err != nil {
			logrus.Error("failed to get auth: ", err)
			return err
		}

		api, err := helix.NewClient(&helix.Options{
			ClientID:       gCtx.Config().Twitch.ClientID,
			ClientSecret:   gCtx.Config().Twitch.ClientSecret,
			RedirectURI:    gCtx.Config().Twitch.RedirectURI,
			AppAccessToken: tkn,
		})
		if err != nil {
			logrus.Fatal("failed to make twitch client: ", err)
		}

		twitchToken := c.Query("state")

		if twitchToken == "" {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, missing state paramater.",
			})
		}

		if twitchToken != c.Cookies("twitch_csrf") {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, csrf_token token missmatch.",
			})
		}

		tknResp, err := api.RequestUserAccessToken(c.Query("code"))
		if err != nil {
			logrus.Errorf("twitch, err=%e", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		api.SetUserAccessToken(tknResp.Data.AccessToken)

		users, err := api.GetUsers(&helix.UsersParams{})
		if err != nil || users.Error != "" || len(users.Data.Users) != 1 {
			if err == nil {
				err = fmt.Errorf("%s %s %d", users.Error, users.ErrorMessage, users.ErrorStatus)
			}
			logrus.Errorf("twitch, err=%e", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
				"error":   err.Error(),
			})
		}

		user := users.Data.Users[0]

		wh := structures.WebHook{}

		res := gCtx.Inst().Mongo.Collection(mongo.CollectionNameWebhooks).FindOneAndDelete(c.Context(), bson.M{
			"user_id": user.ID,
		})

		err = res.Err()
		if err == nil {
			err = res.Decode(&wh)
		}

		if err != nil && err != mongo.ErrNoDocuments {
			logrus.Errorf("mongo, err=%v", err)
			return err
		} else if err == nil {
			_, err = api.RemoveEventSubSubscription(wh.TwitchID)
			if err != nil {
				logrus.Errorf("api err=%v", err)
				return err
			}
		}

		api.SetUserAccessToken("")

		resp, err := api.CreateEventSubSubscription(&helix.EventSubSubscription{
			Type:    "channel.channel_points_custom_reward_redemption.add",
			Version: "1",
			Condition: helix.EventSubCondition{
				BroadcasterUserID: user.ID,
			},
			Transport: helix.EventSubTransport{
				Method:   "webhook",
				Callback: fmt.Sprintf("%s/webhook/%s", gCtx.Config().Frontend.WebsiteURL, user.ID),
				Secret:   gCtx.Config().Twitch.WebhookSecret,
			},
		})
		if err != nil || resp.Error != "" || len(resp.Data.EventSubSubscriptions) == 0 {
			if err == nil {
				err = fmt.Errorf("%s %s %d", resp.Error, resp.ErrorMessage, resp.ErrorStatus)
			}
			logrus.Errorf("api, err=%v", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to create webhooks.",
				"error":   err.Error(),
			})
		}

		_, err = gCtx.Inst().Mongo.Collection(mongo.CollectionNameWebhooks).InsertOne(c.Context(), structures.WebHook{
			TwitchID:  resp.Data.EventSubSubscriptions[0].ID,
			UserID:    user.ID,
			CreatedAt: time.Now(),
		})
		if err != nil {
			logrus.Errorf("mongo, err=%v", err)
			return err
		}

		return c.SendString("All good.")
	})

	app.Post("/webhook/:id", func(c *fiber.Ctx) error {
		streamerID := c.Params("id")

		wh := &structures.WebHook{}
		res := gCtx.Inst().Mongo.Collection(mongo.CollectionNameWebhooks).FindOne(c.Context(), bson.M{
			"user_id": streamerID,
		})
		err := res.Err()
		if err == nil {
			err = res.Decode(wh)
		}
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.SendStatus(404)
			}
			logrus.Errorf("mongo, err=%v", err)
			return err
		}

		t, err := time.Parse(time.RFC3339, c.Get("Twitch-Eventsub-Message-Timestamp"))
		if err != nil || t.Before(time.Now().Add(-10*time.Minute)) {
			return c.SendStatus(400)
		}

		msgID := c.Get("Twitch-Eventsub-Message-Id")

		if msgID == "" {
			return c.SendStatus(400)
		}

		body := c.Body()

		hmacMessage := fmt.Sprintf("%s%s%s", msgID, c.Get("Twitch-Eventsub-Message-Timestamp"), body)

		h := hmac.New(sha256.New, utils.S2B(gCtx.Config().Twitch.WebhookSecret))

		// Write Data to it
		_, err = h.Write(utils.S2B(hmacMessage))
		if err != nil {
			logrus.Errorf("hmac, err=%v", err)
			return c.SendStatus(500)
		}

		// Get result and encode as hexadecimal string
		sha := hex.EncodeToString(h.Sum(nil))

		if c.Get("Twitch-Eventsub-Message-Signature") != fmt.Sprintf("sha256=%s", sha) {
			return c.SendStatus(403)
		}

		newKey := fmt.Sprintf("twitch:events:%s:%s:%s", c.Params("type"), c.Params("id"), msgID)
		set, err := gCtx.Inst().Redis.SetNX(c.Context(), newKey, "1", time.Hour)
		if err != nil {
			logrus.Errorf("redis err=%s", err)
			return c.SendStatus(500)
		}
		if !set {
			logrus.Errorf("duplicate event key=%s", newKey)
			return c.SendStatus(200)
		}

		cleanUp := func(statusCode int, resp string) error {
			if statusCode != 200 {
				if err := gCtx.Inst().Redis.Del(context.Background(), newKey); err != nil {
					logrus.Errorf("redis, err=%e", err)
				}
			}
			if resp == "" {
				return c.SendStatus(statusCode)
			}
			return c.Status(statusCode).SendString(resp)
		}

		callback := WebhookCallback{}
		if err := json.Unmarshal(body, callback); err != nil {
			return cleanUp(400, "")
		}

		if callback.Subscription.Status == "authorization_revoked" {
			return cleanUp(200, "")
		}

		if callback.Challenge != "" {
			return cleanUp(200, callback.Challenge)
		}

		_, err = gCtx.Inst().Mongo.Collection(mongo.CollectionNameRedeemRewards).InsertOne(context.Background(), structures.RedeemEvent{
			TwitchID:   callback.Event.ID,
			RewardID:   callback.Event.Reward.ID,
			RewardName: callback.Event.Reward.Title,
			UserID:     callback.Event.UserID,
			UserName:   callback.Event.UserName,
			Cost:       int32(callback.Event.Reward.Cost),
			RedeemedAt: callback.Event.RedeemedAt.Time,
		})
		if err != nil {
			logrus.Errorf("mongo, err=%v", err)
			return cleanUp(500, "")
		}

		return cleanUp(200, "")
	})
}
