package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"surveyor/surveyor"
	"syscall"
	"time"
)

type flags struct {
	graphsAddr, metricsAddr, urlBase, dataPath string
	interval                                   time.Duration
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	f := parseFlags()

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("server started at %s\n", f.metricsAddr)
		if err := http.ListenAndServe(f.metricsAddr, nil); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server error: %v", err)
		}
	}()

	if err := surveyor.CreateRRD(ctx, f.dataPath, f.interval, f.interval*2); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	server := surveyor.StartGraphServer(f.graphsAddr, f.urlBase, f.dataPath)
	client := surveyor.NewHNAPClient()
	scrapeTicker := time.NewTicker(f.interval)
	defer scrapeTicker.Stop()

loop:
	for {
		run(ctx, client, f.dataPath)

		select {
		case <-ctx.Done():
			break loop
		case <-sigChan:
			fmt.Println("received signal, exiting")
			cancel()
		case <-scrapeTicker.C:
			continue
		}
	}

	if err := server.Shutdown(); err != nil {
		log.Fatalf("error waiting for server to shut down: %v", err)
	}
}

func run(ctx context.Context, client *surveyor.HNAPClient, filename string) {
	// It takes just shy of 3s to get signal data from the modem.
	scrapeCtx, scrapeCancel := context.WithTimeout(ctx, time.Second*5)
	defer scrapeCancel()

	start := time.Now()
	signalData, err := client.GetSignalData(scrapeCtx)
	if err != nil {
		log.Printf("error fetching signal data: %v", err)
		return
	}

	if reportErr := surveyor.ReportSignalData(signalData); reportErr != nil {
		log.Printf("error reporting data: %v", reportErr)
	}

	writeCtx, writeCancel := context.WithTimeout(ctx, time.Second*1)
	defer writeCancel()

	writeErr := surveyor.WriteRRD(writeCtx, filename, start, signalData)
	if writeErr != nil {
		log.Printf("error writing rrd: %v", writeErr)
	}
}

func parseFlags() flags {
	graphsAddr := flag.String("graphsAddr", ":8080", "Listen address for the graph web server")
	urlBase := flag.String("urlBase", "", "URL base for web server")
	dataPath := flag.String("data", "surveyor.rrd", "Path to the RRD database")
	metricsAddr := flag.String("metricsAddr", ":8081", "Listen address for the metrics web server")
	interval := flag.Duration("interval", time.Second*5, "Interval to request metrics from modem")
	flag.Parse()

	f := flags{
		graphsAddr:  *graphsAddr,
		urlBase:     *urlBase,
		dataPath:    *dataPath,
		metricsAddr: *metricsAddr,
		interval:    *interval,
	}
	fmt.Printf(
		"starting surveyor: graphsAddr=%q urlBase=%q data=%q metricsAddr=%q interval=%v\n",
		f.graphsAddr,
		f.urlBase,
		f.dataPath,
		f.metricsAddr,
		f.interval,
	)

	return f
}
