package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

// InitIndex reads the index file from disk and loads the state
// in it's internal map.
func (s *Stream) InitIndex(ctx context.Context) error {
	var (
		dir       = s.dataDir
		indexPath = getIndexPath(dir)
	)

	// Check if data directory exists and report an error if it doesn't.
	ok, err := isExists(dir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("data directory: %s does not exists", dir)
	}

	// Check if index file exists.
	ok, err = isExists(indexPath)
	if err != nil {
		return err
	}

	// If the file doesn't exist, then create a new one with initial state.
	if !ok {
		s.log.Debug("comitting a new index file to", "directory", dir)
		err = s.commitIndex(indexPath)
		if err != nil {
			return err
		}
	} else {
		// Else read and load it to the map.
		s.log.Debug("reading existing index file", "path", indexPath)
		err = s.readIndex(indexPath)
		if err != nil {
			return err
		}
	}

	// Start a background ticker for committing index.
	go s.commitPeriodically(ctx, s.commitInterval)

	return nil
}

// commitIndex writes index data from the map to the disk.
func (s *Stream) commitIndex(path string) error {
	s.Lock()
	defer s.Unlock()

	s.log.Debug("committing index file", "path", path)

	jsonIndex, err := json.Marshal(s.eventIndex)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, jsonIndex, 0644)
	if err != nil {
		return err
	}
	return nil
}

// readIndex reads index file from disk and loads it
// to the map.
func (s *Stream) readIndex(path string) error {
	s.log.Debug("reading index file", "path", path)

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()
	err = json.Unmarshal(data, &s.eventIndex)
	if err != nil {
		return err
	}

	return nil
}

// commitPeriodically is a blocking call. It listens on a ticker channel
// and periodically writes the index state to a file on disk.
// In case context is cancelled it writes the last known index state.
func (s *Stream) commitPeriodically(ctx context.Context, commitInterval time.Duration) {
	var (
		commitTicker = time.NewTicker(commitInterval).C
	)

	s.log.Debug("starting background worker to commit index", "frequency", commitInterval)
	for range commitTicker {
		<-commitTicker
		err := s.commitIndex(getIndexPath(s.dataDir))
		if err != nil {
			s.log.Error("error committing index file", "error", err)
		}
	}
}
