package server

import (
	"time"

	"github.com/AdmiralBulldogTv/BulldogTax/src/global"
	"github.com/AdmiralBulldogTv/BulldogTax/src/mongo"
	"github.com/AdmiralBulldogTv/BulldogTax/src/structures"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

func API(gCtx global.Context, app fiber.Router) {
	app.Get("/tax-results", func(c *fiber.Ctx) error {
		rewardID := c.Query("reward_id")
		start := c.Query("start_date")
		end := c.Query("end_date")
		if start == "" || end == "" || rewardID == "" {
			return c.SendStatus(400)
		}

		startDate, err := time.Parse(time.RFC3339, start)
		if err != nil {
			logrus.Error(err)
			return c.SendStatus(400)
		}
		endDate, err := time.Parse(time.RFC3339, end)
		if err != nil {
			logrus.Error(err)
			return c.SendStatus(400)
		}

		cur, err := gCtx.Inst().Mongo.Collection(mongo.CollectionNameRedeemRewards).Find(c.Context(), bson.M{
			"reward_id": rewardID,
			"redeemed_at": bson.M{
				"$gte": startDate,
				"$lte": endDate,
			},
		})

		results := []structures.RedeemEvent{}
		if err == nil {
			err = cur.All(c.Context(), &results)
		}
		if err != nil {
			logrus.Errorf("mongo, err=%v", err)
			return err
		}

		data, err := json.Marshal(results)
		if err != nil {
			logrus.Errorf("json, err=%v", err)
			return err
		}

		c.Set("Content-Type", "application/json")

		return c.Send(data)
	})
}
