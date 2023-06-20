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

func initProviders(ko *koanf.Koanf, log *logrus.Logger) (providers []provider.Provider, err error) {
	// Initialise HTTP Providers.
	if ko.Exists("http") {
		// Check for legacy sinks.http.root_url key
		if ko.Exists("http.root_url") {
			// Fallback to legacy single provider
			log.Warning("initializing legacy http provider. Move `[sinks.http] to `[sinks.http.default]`")
			log.Warning("only a single sink provider is supported with legacy configuration")
			httpConf := ko.Cut("http")

			options := provider.ParseHTTPOpts("legacy", httpConf)
			options.Log = log

			var http *provider.HTTPManager
			http, err = provider.NewHTTP(options)
			if err != nil {
				err = fmt.Errorf("error initializing legacy provider: %w", err)
				return
			}

			providers = append(providers, http)

			// NOTE: does not parse any additional providers. To use multiple providers one must migrate.
			return
		}

		for _, httpProvider := range ko.MapKeys("http") {
			log.Debugf("initializing http provider %s", httpProvider)
			httpConf := ko.Cut("http." + httpProvider)

			options := provider.ParseHTTPOpts(httpProvider, httpConf)
			options.Log = log

			var http *provider.HTTPManager
			http, err = provider.NewHTTP(options)
			if err != nil {
				err = fmt.Errorf("error initializing provider %s: %w", httpProvider, err)
				return
			}

			providers = append(providers, http)
		}
	}

	return
}

func initSink(ko *koanf.Koanf, log *logrus.Logger) sink.Sink {
	providers, err := initProviders(ko.Cut("sinks"), log)
	if err != nil {
		log.WithError(err).Fatal("error initializing providers")
	}

	var sinkConfig = ko.Cut("sinks")
	if sinkConfig.Exists("batch") {
		// Use legacy batch config
		log.Warning("using legacy sink batch config. Move `[sinks.batch]` under `[sinks]`")
		sinkConfig = ko.Cut("batch")
	}
	sink := sink.New(providers, sink.Opts{
		BatchWorkers:     sinkConfig.Int("workers"),
		BatchQueueSize:   sinkConfig.Int("queue_size"),
		BatchIdleTimeout: sinkConfig.Duration("idle_timeout"),
		BatchEventsCount: sinkConfig.Int("events_count"),
		Log:              log,
	})
	if err != nil {
		log.WithError(err).Fatal("error initialising sink")
	}
	return sink
}

func initStream(ctx context.Context, ko *koanf.Koanf, log *logrus.Logger, cb stream.CallbackFunc) *stream.Stream {
	s, err := stream.New(
		ko.String("app.data_dir"),
		ko.Duration("app.commit_index_interval"),
		cb,
		true,
	)
	if err != nil {
		log.WithError(err).Fatal("error initialising stream")
	}

	return s
}

func initOpts(ko *koanf.Koanf) Opts {
	return Opts{
		topics:               ko.Strings("stream.topics"),
		maxReconnectAttempts: ko.Int("stream.max_reconnect_attempts"),
	}
}
