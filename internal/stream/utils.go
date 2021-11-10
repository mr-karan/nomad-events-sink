package stream

import (
	"os"
	"path/filepath"
)

// getIndexPath returns the file path
// to the index file.
func getIndexPath(dir string) string {
	return filepath.Join(dir, "index.json")
}

// isExists returns boolean indicating whether
// a file or directory exists or not.
func isExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// initEventIndex loads a map with
// topic names and the index of events streamed.
func initEventIndex() map[string]uint64 {
	return map[string]uint64{
		"Deployment": 0,
		"Node":       0,
		"Allocation": 0,
		"Job":        0,
		"Evaluation": 0,
	}
}
