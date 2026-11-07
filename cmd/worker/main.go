package main

import (
	"flag"
	"log"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/app"
)

func main() {
	rebuild := flag.String("rebuild", "", "empty the named projection's read model for the running worker to replay, then exit")

	flag.Parse()

	// Configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	if *rebuild != "" {
		if err = app.RebuildProjection(cfg, *rebuild); err != nil {
			log.Fatalf("Rebuild error: %s", err)
		}

		log.Printf("Projection %s reset: the running worker replays it from the first event", *rebuild)

		return
	}

	if err = app.RunWorker(cfg); err != nil {
		log.Fatalf("Run error: %s", err)
	}
}
