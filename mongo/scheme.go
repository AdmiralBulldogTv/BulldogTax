package mongo

import "time"

type WebHook struct {
	ID        string    `json:"id" bson:"_id"`
	UserID    string    `json:"user_id" bson:"user_id"`
	Secret    string    `json:"secret" bson:"secret"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type RedeemEvent struct {
	ID         string    `json:"-" bson:"_id"`
	RewardID   string    `json:"-" bson:"reward_id"`
	RewardName string    `json:"-" bson:"reward_name"`
	UserID     string    `json:"user_id" bson:"user_id"`
	UserName   string    `json:"-" bson:"user_name"`
	Cost       int32     `json:"cost" bson:"cost"`
	RedeemedAt time.Time `json:"redeemed_at" bson:"redeemed_at"`
}
