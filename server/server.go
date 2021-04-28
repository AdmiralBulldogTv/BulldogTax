package server

import (
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	log "github.com/sirupsen/logrus"
	"github.com/troydota/bulldog-taxes/configure"
	"github.com/troydota/bulldog-taxes/utils"
)

type customLogger struct{}

func (*customLogger) Write(data []byte) (n int, err error) {
	log.Infoln(utils.B2S(data))
	return len(data), nil
}

type Server struct {
	ln  net.Listener
	app *fiber.App
}

func New() *Server {
	ln, err := net.Listen("tcp", configure.Config.GetString("address")) //tls.Listen("tcp", configure.Config.GetString("address"), config)
	if err != nil {
		panic(err)
	}

	server := &Server{
		ln: ln,
		app: fiber.New(fiber.Config{
			ErrorHandler:     errorHandler,
			ReadTimeout:      time.Second,
			DisableKeepalive: true,
		}),
	}
	server.app.Use(recover.New())
	server.app.Use(logger.New(logger.Config{
		Output: &customLogger{},
	}))

	API(server.app)
	Twitch(server.app)

	server.app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(404)
	})

	go func() {
		err = server.app.Listener(server.ln)
		if err != nil {
			log.Errorf("failed to start http server, err=%v", err)
		}
	}()

	return server
}

func errorHandler(c *fiber.Ctx, err error) error {
	log.Errorf("internal err=%v", spew.Sdump(err))

	return c.SendStatus(500)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}
