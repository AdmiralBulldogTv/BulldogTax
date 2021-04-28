package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/troydota/bulldog-taxes/mongo"
	"github.com/troydota/bulldog-taxes/redis"
	"github.com/troydota/bulldog-taxes/utils"

	"github.com/troydota/bulldog-taxes/api"

	"github.com/gofiber/fiber/v2"
	"github.com/troydota/bulldog-taxes/configure"

	"github.com/pasztorpisti/qs"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TwitchTokenResp struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type TwitchCallback struct {
	Challenge    string                     `json:"challenge"`
	Subscription TwitchCallbackSubscription `json:"subscription"`
	Event        map[string]interface{}     `json:"event"`
}

type TwitchCallbackSubscription struct {
	ID        string                  `json:"id"`
	Status    string                  `json:"status"`
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Condition map[string]interface{}  `json:"condition"`
	Transport TwitchCallbackTransport `json:"transport"`
}

type TwitchCallbackTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}
type twitchCSRFPayload struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

func Twitch(app fiber.Router) {
	app.Get("/login", func(c *fiber.Ctx) error {
		csrfToken, err := utils.GenerateRandomString(64)
		if err != nil {
			log.Errorf("secure bytes, err=%e", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		jwt, err := utils.SignJWT(twitchCSRFPayload{
			Token:     csrfToken,
			CreatedAt: time.Now(),
		})
		if err != nil {
			log.Errorf("secure bytes, err=%e", err)
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		scopes := []string{"channel:read:redemptions"}

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"response_type": "code",
			"scope":         strings.Join(scopes, " "),
			"state":         csrfToken,
		})

		u := fmt.Sprintf("https://id.twitch.tv/oauth2/authorize?%s", params)

		c.Cookie(&fiber.Cookie{
			Name:     "twitch_csrf",
			Value:    jwt,
			Domain:   configure.Config.GetString("cookie_domain"),
			Secure:   configure.Config.GetBool("cookie_secure"),
			HTTPOnly: true,
		})

		return c.Redirect(u)
	})

	app.Get("/callback", func(c *fiber.Ctx) error {
		twitchToken := c.Query("state")

		if twitchToken == "" {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, missing state paramater.",
			})
		}

		jwt := strings.Split(c.Cookies("twitch_csrf"), ".")
		if len(jwt) != 3 {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from cookie.",
			})
		}
		jwtPayload := &twitchCSRFPayload{}
		err := utils.VerifyJWT(jwt, jwtPayload)
		if err != nil {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Failed to verify cookie.",
			})
		}

		if twitchToken != jwtPayload.Token {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, csrf_token token missmatch.",
			})
		}

		code := c.Query("code")

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"client_secret": configure.Config.GetString("twitch_client_secret"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"code":          code,
			"grant_type":    "authorization_code",
		})

		u, _ := url.Parse(fmt.Sprintf("https://id.twitch.tv/oauth2/token?%s", params))

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "POST",
			URL:    u,
		})

		if err != nil {
			log.Errorf("twitch, err=%e", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("ioutils, err=%e", err)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		tokenResp := TwitchTokenResp{}

		if err := json.Unmarshal(data, &tokenResp); err != nil {
			log.Errorf("twitch, err=%e, data=%s, url=%s", err, data, u)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		users, err := api.GetUsers(c.Context(), tokenResp.AccessToken, nil, nil)
		if err != nil || len(users) != 1 {
			log.Errorf("twitch, err=%e, resp=%v, token=%v", err, users, tokenResp)
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert access token to user account.",
			})
		}

		user := users[0]

		wh := &mongo.WebHook{}

		res := mongo.Database.Collection("webhooks").FindOneAndDelete(c.Context(), bson.M{
			"user_id": user.ID,
		})
		err = res.Err()
		if err == nil {
			err = res.Decode(wh)
		}
		if err != nil && err != mongo.ErrNoDocuments {
			log.Errorf("mongo, err=%v", err)
			return err
		} else if err == nil {
			err = api.RevokeWebhook(c.Context(), wh.ID)
			if err != nil {
				log.Errorf("api err=%v", err)
				return err
			}
		}

		id, secret, err := api.CreateWebhook(c.Context(), user.ID)
		if err != nil {
			log.Errorf("api, err=%v", err)
			return err
		}

		_, err = mongo.Database.Collection("webhooks").InsertOne(c.Context(), mongo.WebHook{
			ID:        id,
			Secret:    secret,
			UserID:    user.ID,
			CreatedAt: time.Now(),
		})
		if err != nil {
			log.Errorf("mongo, err=%v", err)
			return err
		}

		return c.SendString("All good.")
	})

	app.Post("/webhook/:id", func(c *fiber.Ctx) error {
		streamerID := c.Params("id")

		wh := &mongo.WebHook{}
		res := mongo.Database.Collection("webhooks").FindOne(c.Context(), bson.M{
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
			log.Errorf("mongo, err=%v", err)
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

		h := hmac.New(sha256.New, utils.S2B(wh.Secret))

		// Write Data to it
		_, err = h.Write(utils.S2B(hmacMessage))
		if err != nil {
			log.Errorf("hmac, err=%v", err)
			return c.SendStatus(500)
		}

		// Get result and encode as hexadecimal string
		sha := hex.EncodeToString(h.Sum(nil))

		if c.Get("Twitch-Eventsub-Message-Signature") != fmt.Sprintf("sha256=%s", sha) {
			return c.SendStatus(403)
		}

		newKey := fmt.Sprintf("twitch:events:%s:%s:%s", c.Params("type"), c.Params("id"), msgID)
		err = redis.Client.Do(context.Background(), "SET", newKey, "1", "NX", "EX", 30*60).Err()
		if err != nil {
			if err != redis.ErrNil {
				log.Errorf("redis, err=%e", err)
				return c.SendStatus(500)
			}
			log.Warnf("Duplicated key=%s", newKey)
			return c.SendStatus(200)
		}

		cleanUp := func(statusCode int, resp string) error {
			if statusCode != 200 {
				if err := redis.Client.Del(context.Background(), newKey).Err(); err != nil {
					log.Errorf("refis, err=%e", err)
				}
			}
			if resp == "" {
				return c.SendStatus(statusCode)
			}
			return c.Status(statusCode).SendString(resp)
		}

		callback := &TwitchCallback{}
		if err := json.Unmarshal(body, callback); err != nil {
			return cleanUp(400, "")
		}

		if callback.Subscription.Status == "authorization_revoked" {
			return cleanUp(200, "")
		}

		if callback.Challenge != "" {
			return cleanUp(200, callback.Challenge)
		}

		reward := callback.Event["reward"].(map[string]interface{})

		redeemedAt, err := time.Parse(time.RFC3339, callback.Event["redeemed_at"].(string))
		if err != nil {
			log.Errorf("time, err=%v", err)
			return cleanUp(500, "")
		}

		_, err = mongo.Database.Collection("redeem_events").InsertOne(context.Background(), mongo.RedeemEvent{
			ID:         callback.Event["id"].(string),
			RewardID:   reward["id"].(string),
			RewardName: reward["title"].(string),
			UserID:     callback.Event["user_id"].(string),
			UserName:   callback.Event["user_name"].(string),
			Cost:       int32(reward["cost"].(float64)),
			RedeemedAt: redeemedAt,
		})
		if err != nil {
			log.Errorf("mongo, err=%v", err)
			return cleanUp(500, "")
		}

		return cleanUp(200, "")
	})
}
