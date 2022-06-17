package stream

import (
	"log"
	"os"
)

// Logger provides function helpers for logging messgaes with various log levels.
type logger struct {
	debugf func(format string, args ...any)
	errorf func(format string, args ...any)
}

// initLogger constructs a logger that writes to stdout.
// It logs at the specified log level and above.
// It decorates log lines with the log level, date, time, file.
func initLogger(verbose bool) *logger {
	// Init logger with no-op functions.
	l := &logger{
		debugf: func(format string, args ...any) {},
		errorf: func(format string, args ...any) {},
	}

	// Instantiate log functions for different levels.
	logfn := func(lvl string) func(string, ...any) {
		return log.New(os.Stdout, lvl+":", log.Ldate|log.Ltime|log.Lshortfile).Printf
	}
	if verbose {
		l.debugf = logfn("DEBUG")
	}
	l.errorf = logfn("ERROR")
	return l
}
