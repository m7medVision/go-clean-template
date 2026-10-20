package main

import (
	"log"
	"os"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	// Export
	doc, err := restapi.OpenAPI(cfg.App.Version)
	if err != nil {
		log.Fatalf("OpenAPI error: %s", err)
	}

	if _, err = os.Stdout.Write(append(doc, '\n')); err != nil {
		log.Fatalf("Write error: %s", err)
	}
}
