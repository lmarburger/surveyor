package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"surveyor/surveyor"
	"syscall"
)

type flags struct {
	addr string
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	f := parseFlags()
	client := surveyor.NewHNAPClient()
	collector := surveyor.NewSignalDataCollector(client, prometheus.DefaultRegisterer)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("server started at %s\n", f.addr)
		if err := http.ListenAndServe(f.addr, nil); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server error: %v", err)
		}
	}()

	<-sigChan
	fmt.Println("received signal, exiting")
}

func parseFlags() flags {
	addr := flag.String("addr", ":8080", "Listen address for the metrics web server")
	flag.Parse()

	f := flags{addr: *addr}
	fmt.Printf("starting surveyor: addr=%q\n", f.addr)

	return f
}
