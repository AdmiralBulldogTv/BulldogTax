package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/troydota/bulldog-taxes/configure"
	_ "github.com/troydota/bulldog-taxes/mongo"
	_ "github.com/troydota/bulldog-taxes/redis"
	"github.com/troydota/bulldog-taxes/server"
)

func main() {
	log.Infoln("Application Starting...")

	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		log.Warnf("Invalid exit code specified in config (%v), using 0 as new exit code.", configCode)
		configCode = 0
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	s := server.New()

	go func() {
		sig := <-c
		log.Infof("sig=%v, gracefully shutting down...", sig)
		start := time.Now().UnixNano()

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Shutdown(); err != nil {
				log.Errorf("shutdown failed for server, %v", err)
			}
		}()
		wg.Wait()

		log.Infof("Shutdown took, %.2fms", float64(time.Now().UnixNano()-start)/10e5)
		os.Exit(configCode)
	}()

	log.Infoln("Application Started.")

	select {}
}
