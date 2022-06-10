package stream

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/sirupsen/logrus"
)

type CallbackFunc func(api.Event)

type Stream struct {
	sync.RWMutex
	log            *logrus.Logger
	client         *api.Client
	eventIndex     map[string]uint64
	dataDir        string
	commitInterval time.Duration
	callback       CallbackFunc
}

// New initialises a Stream object.
func New(log *logrus.Logger, callback CallbackFunc, dir string, commitInterval time.Duration) (*Stream, error) {
	// Initialise a Nomad API client with default config.
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &Stream{
		client:         client,
		log:            log,
		callback:       callback,
		dataDir:        dir,
		eventIndex:     initEventIndex(),
		commitInterval: commitInterval,
	}, nil
}

// Subscribe establishes a subscription to Nomad's
// Event streaming channel and pushes new messages
// to the Sink channel for further processing.
func (s *Stream) Subscribe(ctx context.Context, topic string, maxReconnectAttempts int) error {
	if maxReconnectAttempts == -1 {
		maxReconnectAttempts = int(math.MaxInt64)
	}

	attempt := 0

	for {
		// If max retries reached, report error.
		if attempt == maxReconnectAttempts {
			return fmt.Errorf("max reconnect attempts reached")
		}

		// Pause before reconnecting again.
		if attempt > 0 {
			time.Sleep(time.Second * 3)
		}

		eventCh, err := s.initStreamChannel(ctx, topic)
		if err != nil {
			// If context is cancelled, exit gracefully.
			if ctx.Err() != nil {
				return nil
			}
			// Else try connecting to stream again.
			attempt++
			s.log.WithField("topic", topic).WithField("attempt", attempt).WithField("remaining", maxReconnectAttempts-attempt).Warn("attempting to reconnect to stream")
			continue
		}

		// Once the channel is initialised, start reading events.
		err = s.handleEvents(ctx, eventCh)
		if err != nil {
			s.log.WithError(err).Error("error handling events")
			continue
		}
		return nil
	}
}

// initStreamChannel initialises a new events channel for a given topic.
func (s *Stream) initStreamChannel(ctx context.Context, topic string) (<-chan *api.Events, error) {
	topics := map[api.Topic][]string{
		api.Topic(topic): {"*"},
	}

	// Get the last index processed for the given topic.
	s.RLock()
	index := s.eventIndex[topic]
	s.RUnlock()

	s.log.WithFields(logrus.Fields{
		"topic": api.Topic(topic),
		"index": index,
	}).Info("subscribing to stream")

	events := s.client.EventStream()
	eventCh, err := events.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		s.log.WithError(err).Error("error initialising stream client")
		return nil, err
	}
	return eventCh, nil
}

// handleEvents reads events from the events channel and adds to sink for further processing.
func (s *Stream) handleEvents(ctx context.Context, eventCh <-chan *api.Events) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-eventCh:
			if event.Err != nil {
				s.log.WithError(event.Err).Error("error consuming event")
				return event.Err
			}

			// Skip heartbeat events.
			if event.IsHeartbeat() {
				continue
			}

			// Call the callback func.
			for _, e := range event.Events {
				s.callback(e)
			}

			// Write the latest index to the map.
			last := event.Events[len(event.Events)-1]
			s.Lock()
			s.eventIndex[string(last.Topic)] = last.Index
			s.Unlock()
		}
	}
}
