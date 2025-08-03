package main

import (
	"context"
	"pipeliner/pkg/engine"

	log "github.com/sirupsen/logrus"
)

func main() {

	ctx := context.Background()
	engine := engine.NewEngine(ctx)

	log.Info("Pipeliner Engine is starting...")

	if err := engine.Run(); err != nil {
		log.Errorf("Error running the engine: %v", err)
	}

	log.Info("Pipeliner Engine has finished running.")
}
