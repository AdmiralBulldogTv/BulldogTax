package redis

import (
	"github.com/go-redis/redis/v8"
	"github.com/troydota/bulldog-taxes/configure"
)

var Client *redis.Client

func init() {
	options, err := redis.ParseURL(configure.Config.GetString("redis_uri"))
	if err != nil {
		panic(err)
	}

	Client = redis.NewClient(options)
}

type Message = redis.Message

const ErrNil = redis.Nil

type StringCmd = redis.StringCmd

type Pipeliner = redis.Pipeliner
