package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"surveyor/surveyor"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type config struct {
	addr       string
	modemURL   string
	username   string
	password   string
	interval   time.Duration
	timeout    time.Duration
	maxBackoff time.Duration
}

func main() {
	if err := run(); err != nil {
		slog.Error("surveyor exiting", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	slog.Info("starting surveyor", "addr", cfg.addr, "modem_url", cfg.modemURL,
		"interval", cfg.interval, "timeout", cfg.timeout, "max_backoff", cfg.maxBackoff)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := surveyor.NewHNAPClient(cfg.modemURL, cfg.username, cfg.password)
	poller := surveyor.NewPoller(client, surveyor.PollerConfig{
		Interval:   cfg.interval,
		Timeout:    cfg.timeout,
		MaxBackoff: cfg.maxBackoff,
	}, prometheus.DefaultRegisterer)
	prometheus.MustRegister(surveyor.NewCollector(poller, 3*cfg.interval))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: cfg.addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("metrics server listening", "addr", cfg.addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	pollerDone := make(chan struct{})
	go func() {
		poller.Run(ctx)
		close(pollerDone)
	}()

	select {
	case err := <-serverErr:
		stop()
		<-pollerDone
		return fmt.Errorf("metrics server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("received signal, shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(shutdownCtx)
	<-pollerDone
	return err
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	fs := flag.NewFlagSet("surveyor", flag.ContinueOnError)
	cfg := config{password: getenv("SURVEYOR_MODEM_PASSWORD")}

	fs.StringVar(&cfg.addr, "addr", ":8080", "Listen address for the metrics server")
	fs.StringVar(&cfg.modemURL, "modem-url", envOr(getenv, "SURVEYOR_MODEM_URL", surveyor.DefaultURL),
		"Modem HNAP endpoint (env SURVEYOR_MODEM_URL)")
	fs.StringVar(&cfg.username, "modem-username", envOr(getenv, "SURVEYOR_MODEM_USERNAME", surveyor.DefaultUsername),
		"Modem admin username (env SURVEYOR_MODEM_USERNAME)")
	fs.DurationVar(&cfg.interval, "interval", 30*time.Second, "Time between modem polls")
	fs.DurationVar(&cfg.timeout, "timeout", 15*time.Second, "Deadline for one poll, including login")
	fs.DurationVar(&cfg.maxBackoff, "max-backoff", 5*time.Minute, "Longest wait between polls while the modem is failing")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: surveyor [flags]")
		fmt.Fprintln(fs.Output(), "The modem password is read from SURVEYOR_MODEM_PASSWORD.")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.password == "" {
		return config{}, errors.New("SURVEYOR_MODEM_PASSWORD is not set")
	}
	if cfg.interval <= 0 || cfg.timeout <= 0 {
		return config{}, errors.New("-interval and -timeout must be positive")
	}
	if cfg.maxBackoff < cfg.interval {
		return config{}, fmt.Errorf("-max-backoff (%s) must be at least -interval (%s)", cfg.maxBackoff, cfg.interval)
	}
	return cfg, nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}
