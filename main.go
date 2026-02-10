package main

import (
	"log"
	"os"

	"omniq-monitoring/internal/config"
	"omniq-monitoring/internal/omniq"
	"omniq-monitoring/internal/ui"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client, err := omniq.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer client.Close()

	monitor := omniq.NewMonitor(client)

	app := ui.NewApp(cfg, client, monitor)

	if err := app.Run(); err != nil {
		client.Close()
		log.Printf("Application error: %v", err)
		os.Exit(1)
	}
}
