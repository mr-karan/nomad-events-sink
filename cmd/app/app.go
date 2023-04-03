package main

import (
	"context"
	"sync"

	sink "github.com/mr-karan/nomad-events-sink/internal/sinks"
	"github.com/mr-karan/nomad-events-sink/pkg/stream"
	"golang.org/x/exp/slog"
)

type Opts struct {
	maxReconnectAttempts int
	topics               []string
}

// App is the global container that holds
// objects of various routines that run on boot.
type App struct {
	log    *slog.Logger
	stream *stream.Stream
	sink   sink.Sink
	opts   Opts
}

// Start initialises the sink workers and
// subscription stream in background and waits
// for context to be cancelled to exit.
func (app *App) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}

	// Spawn sink workers that process incoming events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.sink.Run(ctx)
	}()

	// Initialise index store from disk to continue reading
	// from last event which is processed.
	err := app.stream.InitIndex(ctx)
	if err != nil {
		app.log.Error("error initialising index store", "error", err)
	}

	for _, t := range app.opts.topics {
		wg.Add(1)
		topic := t
		go func() {
			defer wg.Done()
			// Subscribe to events.
			if err := app.stream.Subscribe(ctx, topic, app.opts.maxReconnectAttempts); err != nil {
				app.log.Error("error subscribing to events", "error", err, "topic", topic)
			}
		}()
	}
	// Wait for all routines to finish.
	wg.Wait()
}
