package global

import "github.com/AdmiralBulldogTv/BulldogTax/src/instance"

type Instances struct {
	Redis instance.Redis
	Mongo instance.Mongo
}
