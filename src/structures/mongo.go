package structures

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WebHook struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TwitchID  string             `json:"twitch_id" bson:"twitch_id"`
	UserID    string             `json:"user_id" bson:"user_id"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
}

type RedeemEvent struct {
	ID         primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	TwitchID   string             `json:"-" bson:"twitch_id"`
	RewardID   string             `json:"-" bson:"reward_id"`
	RewardName string             `json:"-" bson:"reward_name"`
	UserID     string             `json:"user_id" bson:"user_id"`
	UserName   string             `json:"-" bson:"user_name"`
	Cost       int32              `json:"cost" bson:"cost"`
	RedeemedAt time.Time          `json:"redeemed_at" bson:"redeemed_at"`
}
