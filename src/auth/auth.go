package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/AdmiralBulldogTv/BulldogTax/src/global"
	"github.com/nicklaw5/helix"
	"github.com/sirupsen/logrus"
)

var ErrInvalidRespTwitch = fmt.Errorf("invalid resp from twitch")

func GetAuth(gCtx global.Context, ctx context.Context) (string, error) {
	val, err := gCtx.Inst().Redis.Get(ctx, "twitch:auth")
	if err != nil {
		logrus.Warn("unable to get auth from redis: ", err)
	} else {
		return val.(string), nil
	}

	api, err := helix.NewClient(&helix.Options{
		ClientID:     gCtx.Config().Twitch.ClientID,
		ClientSecret: gCtx.Config().Twitch.ClientSecret,
	})
	if err != nil {
		return "", err
	}

	tkn, err := api.RequestAppAccessToken(nil)
	if err != nil {
		return "", err
	}

	auth := tkn.Data.AccessToken

	expiry := time.Second * time.Duration(int64(float64(tkn.Data.ExpiresIn)*0.75))

	if err := gCtx.Inst().Redis.SetEX(ctx, "twitch:auth", auth, expiry); err != nil {
		logrus.Errorf("redis, err=%e", err)
	}

	return auth, nil
}
