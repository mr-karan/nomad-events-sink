package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	sink "github.com/mr-karan/nomad-events-sink/internal/sinks"
	"github.com/mr-karan/nomad-events-sink/internal/sinks/provider"
	"github.com/mr-karan/nomad-events-sink/pkg/stream"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

// initLogger initializes logger.
func initLogger(ko *koanf.Koanf) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})
	if ko.String("app.log") == "debug" {
		logger.SetLevel(logrus.DebugLevel)
	}
	return logger
}

// initConfig loads config to `ko`
// object.
func initConfig(cfgDefault string, envPrefix string) (*koanf.Koanf, error) {
	var (
		ko = koanf.New(".")
		f  = flag.NewFlagSet("front", flag.ContinueOnError)
	)

	// Configure Flags.
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	// Register `--config` flag.
	cfgPath := f.String("config", cfgDefault, "Path to a config file to load.")

	// Parse and Load Flags.
	err := f.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	// Load the config files from the path provided.
	err = ko.Load(file.Provider(*cfgPath), toml.Parser())
	if err != nil {
		return nil, err
	}

	// Load environment variables if the key is given
	// and merge into the loaded config.
	if envPrefix != "" {
		err = ko.Load(env.Provider(envPrefix, ".", func(s string) string {
			return strings.Replace(strings.ToLower(
				strings.TrimPrefix(s, envPrefix)), "__", ".", -1)
		}), nil)
		if err != nil {
			return nil, err
		}
	}

	return ko, nil
}

func initSink(ko *koanf.Koanf, log *logrus.Logger) sink.Sink {
	// Initialise HTTP Provider.
	http, err := provider.NewHTTP(
		provider.HTTPOpts{
			Log:                log,
			RootURL:            ko.String("sinks.http.root_url"),
			Timeout:            ko.Duration("sinks.http.timeout"),
			MaxConnections:     ko.Int("sinks.http.max_idle_conns"),
			HealthCheckEnabled: ko.Bool("sinks.http.healthcheck.enabled"),
			HealthcheckURL:     ko.String("sinks.http.healthcheck.url"),
			HealthCheckStatus:  ko.Int("sinks.http.healthcheck.status"),
		})
	if err != nil {
		log.WithError(err).Fatal("error initialising http sink provider")
	}

	sink := sink.New([]provider.Provider{http}, sink.Opts{
		BatchWorkers:     ko.Int("sinks.batch.workers"),
		BatchQueueSize:   ko.Int("sinks.batch.queue_size"),
		BatchIdleTimeout: ko.Duration("sinks.batch.idle_timeout"),
		BatchEventsCount: ko.Int("sinks.batch.events_count"),
		Log:              log,
	})
	if err != nil {
		log.WithError(err).Fatal("error initialising sink")
	}
	return sink
}

func initStream(ctx context.Context, ko *koanf.Koanf, log *logrus.Logger, cb stream.CallbackFunc) *stream.Stream {
	s, err := stream.New(log,
		ko.String("app.data_dir"),
		ko.Duration("app.commit_index_interval"),
	)
	if err != nil {
		log.WithError(err).Fatal("error initialising stream")
	}

	// Set callback for processing events.
	s.SetCB(cb)
	return s
}

func initOpts(ko *koanf.Koanf) Opts {
	return Opts{
		topics:               ko.Strings("stream.topics"),
		maxReconnectAttempts: ko.Int("stream.max_reconnect_attempts"),
	}
}
