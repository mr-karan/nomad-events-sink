package sink

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mr-karan/nomad-events-sink/internal/sinks/provider"
	"golang.org/x/exp/slog"
)

// Sink represents the configuration to process events
// with various upstream providers.
type Sink struct {
	log      *slog.Logger
	finished bool
	workers  []Worker
	Opts     Opts
	Queue    chan api.Event
}

type Opts struct {
	Log              *slog.Logger
	BatchWorkers     int
	BatchQueueSize   int
	BatchIdleTimeout time.Duration
	BatchEventsCount int
}

// New initialises sink workers which process
// events and dump to external sources using the configured
// providers.
func New(provs []provider.Provider, opt Opts) Sink {
	// Configure sane defaults.
	if opt.BatchWorkers == 0 {
		opt.BatchWorkers = 2
	}
	if opt.BatchQueueSize == 0 {
		opt.BatchQueueSize = 10
	}
	if opt.BatchIdleTimeout == 0 {
		opt.BatchIdleTimeout = time.Second * 5
	}
	if opt.BatchEventsCount == 0 {
		opt.BatchEventsCount = 5
	}

	// Initialise workers.
	workers := make([]Worker, 0, opt.BatchWorkers)
	for i := 0; i < opt.BatchWorkers; i++ {
		w := initWorker(provs, opt)
		workers = append(workers, w)
	}

	return Sink{
		log:      opt.Log,
		finished: false,
		workers:  workers,
		Opts:     opt,
		Queue:    make(chan api.Event, opt.BatchQueueSize),
	}
}

// Run listens on the events queue
// and produces a batch of events which are consumed by workers.
// It is a blocking call.
func (s *Sink) Run(ctx context.Context) {
	wg := &sync.WaitGroup{}

	s.log.Info("starting to consume events from the events queue", "workers", s.Opts.BatchWorkers)
	for i := 0; i < s.Opts.BatchWorkers; i++ {
		wg.Add(1)
		go func() {
			s.workers[i].processEvents(ctx, s.Queue)
			wg.Done()
		}()
		wg.Wait()
	}
}

// Add adds a new event to the sink queue.
func (s *Sink) Add(event api.Event) {
	if s.finished {
		return
	}
	s.Queue <- event
}
