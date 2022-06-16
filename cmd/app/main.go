package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/nomad/api"
	"github.com/mr-karan/nomad-events-sink/pkg/stream"
)

var (
	// Version of the build. This is injected at build-time.
	buildString = "unknown"
)

func main() {
	// Create a new context which gets cancelled upon receiving `SIGINT`/`SIGTERM`.
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Initialise and load the config.
	ko, err := initConfig("config.sample.toml", "NOMAD_EVENTS_SINK_")
	if err != nil {
		fmt.Println("error initialising config", err)
		os.Exit(1)
	}

	var (
		log    = initLogger(ko)
		sink   = initSink(ko, log)
		stream = initStream(ctx, ko, log, func(e api.Event, meta stream.Meta) {
			sink.Add(e)
		})
		opts = initOpts(ko)
	)

	// Initialise a new instance of app.
	app := App{
		log:    log,
		sink:   sink,
		stream: stream,
		opts:   opts,
	}

	// Start an instance of app.
	app.log.WithField("version", buildString).Info("booting nomad events collector")
	app.Start(ctx)
}
