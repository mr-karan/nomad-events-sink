package sink

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mr-karan/nomad-events-sink/internal/sinks/provider"
	"golang.org/x/exp/slog"
)

type Worker struct {
	log              *slog.Logger
	batch            []api.Event
	providers        []provider.Provider
	batchIdleTimeout time.Duration
	batchEventsCount int
}

// initWorker creates a worker object.
func initWorker(provs []provider.Provider, opt Opts) Worker {
	return Worker{
		providers:        provs,
		log:              opt.Log,
		batch:            make([]api.Event, 0),
		batchIdleTimeout: opt.BatchIdleTimeout,
		batchEventsCount: opt.BatchEventsCount,
	}
}

// processEvents listens to incoming events and batches
// before sening to upstream providers.
func (w *Worker) processEvents(ctx context.Context, queue chan api.Event) {
	// Create a batchTicker to batch events when `batch_max_idle_time` is reached.
	batchTicker := time.NewTicker(w.batchIdleTimeout).C

	for {
		select {
		case <-ctx.Done():
			// Context is cancelled, flush remaining batches.
			w.flush(w.batch)
			return

		case e, ok := <-queue:
			if !ok {
				// Queue is closed, flush remaining batches.
				w.flush(w.batch)
				return
			}

			w.batch = append(w.batch, e)

			// If events slice reached the max event count for a batch,
			// add to batch.
			if len(w.batch) == w.batchEventsCount {
				w.flush(w.batch)
				// Reset the batch after flushing events.
				w.batch = []api.Event{}
			}

		case <-batchTicker:
			w.flush(w.batch)
			// Reset the batch after flushing events.
			w.batch = []api.Event{}
		}
	}
}

// flush takes a batch of events and pushes to
// upstream providers.
func (w *Worker) flush(batch []api.Event) {
	if len(batch) == 0 {
		return
	}

	data, err := prepareJSON(batch)
	if err != nil {
		w.log.Error("error while json marshall", "error", err, "batch_len", len(batch))
	}

	w.log.Info("pushing events to providers", "batch_len", len(batch))
	for _, prov := range w.providers {
		err = prov.Push(data)
		if err != nil {
			// TODO: Handle the error better.
			w.log.Error("error while pushing to provider", "error", err)
		}
	}
}

// prepareJSON takes batches of events and returns JSON encoding of the same.
func prepareJSON(events []api.Event) ([]byte, error) {
	data, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	return data, nil
}
