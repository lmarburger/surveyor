package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"surveyor/surveyor"
	"syscall"
	"time"
)

type flags struct {
	addr, urlBase, dataPath string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	f := parseFlags()
	step := time.Second * 5

	scrapeTicker := time.NewTicker(step)
	defer scrapeTicker.Stop()

	rrdErr := surveyor.CreateRRD(ctx, f.dataPath, step, step*2)
	if rrdErr != nil {
		fmt.Println(rrdErr)
		os.Exit(1)
	}

	server := surveyor.StartGraphServer(f.addr, f.urlBase, f.dataPath)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-sigChan:
			fmt.Println("received signal, exiting")
			cancel()
		case <-scrapeTicker.C:
			run(ctx, f.dataPath)
		}
	}

	err := server.Shutdown()
	if err != nil {
		log.Fatalf("error waiting for server to shut down: %v", err)
	}
}

func run(ctx context.Context, filename string) {
	scrapeCtx, scrapeCancel := context.WithTimeout(ctx, time.Second*2)
	defer scrapeCancel()

	start := time.Now()
	signalData, scrapeErr := surveyor.Scrape(scrapeCtx)
	if scrapeErr != nil {
		log.Printf("error scraping: %v", scrapeErr)
		return
	}

	writeCtx, writeCancel := context.WithTimeout(ctx, time.Second*1)
	defer writeCancel()

	writeErr := surveyor.WriteRRD(writeCtx, filename, start, signalData)
	if writeErr != nil {
		log.Printf("error writing rrd: %v", writeErr)
	}
}

func parseFlags() flags {
	addr := flag.String("addr", ":8080", "Listen address for the web server")
	urlBase := flag.String("urlBase", "", "URL base for web server")
	dataPath := flag.String("data", "surveyor.rrd", "Path to the RRD database")
	flag.Parse()

	f := flags{addr: *addr, urlBase: *urlBase, dataPath: *dataPath}
	fmt.Printf("starting surveyor: addr=%q urlBase=%q data=%q\n", f.addr, f.urlBase, f.dataPath)

	return f
}
