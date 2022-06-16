package stream

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
)

type Meta struct {
	NodeID string
}

type CallbackFunc func(api.Event, Meta)

type Stream struct {
	sync.RWMutex
	log            *logger
	client         *api.Client
	eventIndex     map[string]uint64
	dataDir        string
	commitInterval time.Duration
	callback       CallbackFunc
}

// New initialises a Stream object.
func New(dir string, commitInterval time.Duration, cb CallbackFunc, verbose bool) (*Stream, error) {
	// Initialise a Nomad API client with default config.
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	// Check if callback is not nil
	if cb == nil {
		return nil, fmt.Errorf("callback can't be nil")
	}

	return &Stream{
		log:            initLogger(verbose),
		client:         client,
		dataDir:        dir,
		eventIndex:     initEventIndex(),
		commitInterval: commitInterval,
		callback:       cb,
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
			s.log.errorf("attempting to reconnect to stream on topic: %s, attempt: %d, remaining: %d", topic, attempt, maxReconnectAttempts)
			continue
		}

		// Once the channel is initialised, start reading events.
		err = s.handleEvents(ctx, eventCh)
		if err != nil {
			s.log.errorf("error handling events: %v", err)
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

	s.log.debugf("subscribing to stream on topic %s from index %d", api.Topic(topic), index)

	events := s.client.EventStream()
	eventCh, err := events.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		s.log.errorf("error initialising stream client: %v", err)
		return nil, err
	}
	return eventCh, nil
}

// handleEvents reads events from the events channel and adds to sink for further processing.
func (s *Stream) handleEvents(ctx context.Context, eventCh <-chan *api.Events) error {
	nodeID, err := s.nodeID()
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			s.log.debugf("cancellation signal received; comitting index file")
			err := s.commitIndex(getIndexPath(s.dataDir))
			if err != nil {
				s.log.errorf("error committing index file: %v", err)
			}
			return nil
		case event := <-eventCh:
			if event.Err != nil {
				s.log.errorf("error consuming event: %v", err)
				return event.Err
			}

			// Skip heartbeat events.
			if event.IsHeartbeat() {
				continue
			}

			// Call the callback func.
			for _, e := range event.Events {
				if s.callback != nil {
					s.callback(e, Meta{
						NodeID: nodeID,
					})
				}
			}

			// Write the latest index to the map.
			last := event.Events[len(event.Events)-1]
			s.Lock()
			s.eventIndex[string(last.Topic)] = last.Index
			s.Unlock()
		}
	}
}

// nodeID Returns the NodeID of the underlying Nomad client it's running on.
func (s *Stream) nodeID() (string, error) {
	self, err := s.client.Agent().Self()
	if err != nil {
		return "", fmt.Errorf("unable to fetch node: %v", err)
	}
	return self.Stats["client"]["node_id"], nil
}
