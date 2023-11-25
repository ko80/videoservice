package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"

	"videoservice/api"
	"videoservice/framer"
	"videoservice/storage"
)

type Config struct {
	API     api.Config
	Framer  framer.Config
	Storage storage.Config
}

func main() {
	// Detect environment
	env := os.Getenv("CONTAINER_ENVIRONMENT")
	if env == "" {
		env = "local"
	}

	// Load config
	var cfg Config
	cfgPath := fmt.Sprintf("config-%s.toml", env)
	_, err := toml.DecodeFile(cfgPath, &cfg)
	if err != nil {
		logrus.Fatalf("Can't load config file: %s %v", cfgPath, err)
	}

	// Setup logger
	if cfg.API.IsProd {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	} else {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	}

	// Setup logger for http standard output
	log.SetOutput(logrus.StandardLogger().Writer())

	logrus.Info("Service started")

	ctx, cancel := context.WithCancel(context.Background())

	frm := framer.NewFramer(cfg.Framer)
	sto := storage.NewStorage(cfg.Storage)

	videoApi, err := api.NewAPI(cfg.API, frm, sto)
	if err != nil {
		logrus.Fatalf("VideoAPI init err: %v, cfg: %v", err, cfg.API)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		frm.Run(ctx)
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		videoApi.Run(ctx)
		cancel()
	}()

	// Start a goroutine for handling shutdown signals
	go func() {
		// setup notification of shutdown signals
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

		select {
		// quit cycle if signals fired
		case <-signals:
		// quit cycle if the main context is closed
		case <-ctx.Done():
		}

		logrus.Info("Shutdown signal received")
		cancel()
	}()

	wg.Wait()
	logrus.Info("Service exited gracefully")
}
