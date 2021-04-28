package mongo

import (
	"context"

	"github.com/troydota/bulldog-taxes/configure"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Database *mongo.Database

var ErrNoDocuments = mongo.ErrNoDocuments

func init() {
	ctx := context.TODO()
	clientOptions := options.Client().ApplyURI(configure.Config.GetString("mongo_uri"))
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		panic(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		panic(err)
	}

	Database = client.Database(configure.Config.GetString("mongo_db"))

	_, err = Database.Collection("webhooks").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"user_id": 1}, Options: options.Index().SetUnique(true)},
	})
	if err != nil {
		panic(err)
	}

	_, err = Database.Collection("redeem_events").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"reward_id": 1}},
		{Keys: bson.M{"user_id": 1}},
		{Keys: bson.M{"redeemed_at": 1}},
	})
	if err != nil {
		panic(err)
	}
}
