package server

import (
	"time"

	"github.com/AdmiralBulldogTv/BulldogTax/src/global"
	"github.com/AdmiralBulldogTv/BulldogTax/src/utils"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/sirupsen/logrus"
)

type customLogger struct{}

func (*customLogger) Write(data []byte) (n int, err error) {
	logrus.Infoln(utils.B2S(data))
	return len(data), nil
}

func New(gCtx global.Context) <-chan struct{} {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logrus.Errorf("internal err=%v", spew.Sdump(err))

			return c.SendStatus(500)
		},
		ReadTimeout:           time.Second * 10,
		WriteTimeout:          time.Second * 10,
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Output: &customLogger{},
	}))

	API(gCtx, app)
	Twitch(gCtx, app)

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(404)
	})

	done := make(chan struct{})
	go func() {
		if err := app.Listen(gCtx.Config().API.Bind); err != nil {
			logrus.Errorf("failed to start http server, err=%v", err)
		}
		close(done)
	}()

	go func() {
		<-gCtx.Done()
		_ = app.Shutdown()
	}()

	return done
}
