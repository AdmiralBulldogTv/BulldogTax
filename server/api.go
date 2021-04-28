package server

import (
	"time"

	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"github.com/troydota/bulldog-taxes/mongo"
	"go.mongodb.org/mongo-driver/bson"
)

func API(app fiber.Router) {
	app.Get("/tax-results", func(c *fiber.Ctx) error {
		rewardID := c.Query("reward_id")
		start := c.Query("start_date")
		end := c.Query("end_date")
		if start == "" || end == "" || rewardID == "" {
			return c.SendStatus(400)
		}

		startDate, err := time.Parse(time.RFC3339, start)
		if err != nil {
			log.Error(err)
			return c.SendStatus(400)
		}
		endDate, err := time.Parse(time.RFC3339, end)
		if err != nil {
			log.Error(err)
			return c.SendStatus(400)
		}

		cur, err := mongo.Database.Collection("redeem_events").Find(c.Context(), bson.M{
			"reward_id": rewardID,
			"redeemed_at": bson.M{
				"$gte": startDate,
				"$lte": endDate,
			},
		})

		results := []mongo.RedeemEvent{}
		if err == nil {
			err = cur.All(c.Context(), &results)
		}
		if err != nil {
			log.Errorf("mongo, err=%v", err)
			return err
		}

		data, err := json.Marshal(results)
		if err != nil {
			log.Errorf("json, err=%v", err)
			return err
		}

		c.Set("Content-Type", "application/json")

		return c.Send(data)
	})
}
