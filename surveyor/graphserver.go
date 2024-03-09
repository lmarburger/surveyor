package surveyor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type GraphServer struct {
	server *http.Server
}

func StartGraphServer(addr, urlBase, dataPath string) GraphServer {
	server := NewGraphServer(addr, urlBase, dataPath)
	server.Serve()
	return server
}

type GraphHandler struct {
	dataPath string
}

func NewGraphServer(addr, urlBase, dataPath string) GraphServer {
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle(fmt.Sprintf("%s/", urlBase), http.StripPrefix(urlBase, fs))

	handler := GraphHandler{dataPath}
	timeout := time.Second * 2
	serverHandler := http.TimeoutHandler(http.HandlerFunc(handler.graphHandler), timeout, "Timeout\n")
	graphsPath := fmt.Sprintf("%s/graphs/", urlBase)
	mux.Handle(graphsPath, http.StripPrefix(graphsPath, serverHandler))

	return GraphServer{
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			WriteTimeout: timeout * 2,
		},
	}
}

func (srv GraphServer) Serve() {
	go func() {
		log.Printf("server started at %s\n", srv.server.Addr)
		if err := srv.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()
}

func (srv GraphServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	if err := srv.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down server: %v", err)
	}

	return nil
}

func (handler GraphHandler) graphHandler(writer http.ResponseWriter, request *http.Request) {
	log.Printf("received request: %q", request.URL.Path)
	if request.URL.Path == "/favicon.ico" {
		http.Error(writer, "Error parsing path", http.StatusNotFound)
		return
	}

	details, err := parsePath(request.URL.Path)
	if err != nil {
		http.Error(writer, "Error parsing path", http.StatusInternalServerError)
		return
	}

	graphPath, err := handler.generateGraph(request.Context(), details)
	if err != nil {
		http.Error(writer, "Failed to generate graph", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := os.Remove(graphPath); err != nil {
			log.Printf("error removing file: %v\n", err)
		}
	}()

	writer.Header().Set("Content-Type", "image/png")
	graphFile, err := os.Open(graphPath)
	if err != nil {
		http.Error(writer, "Failed to open graph", http.StatusInternalServerError)
		return
	}
	defer ClosePrintErr(graphFile)

	_, err = io.Copy(writer, graphFile) // Serve the file.
	if err != nil {
		http.Error(writer, "Failed to serve graph", http.StatusInternalServerError)
		return
	}
}

func (handler GraphHandler) generateGraph(ctx context.Context, details RenderDetails) (string, error) {
	file, err := os.CreateTemp("", fmt.Sprintf("graph-%s-*.png", details.data))
	if err != nil {
		return "", err
	}

	filePath := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}

	err = WriteGraph(ctx, handler.dataPath, filePath, details)
	if err != nil {
		log.Printf("error writing graph: %v\n", err)
		if err := os.Remove(filePath); err != nil {
			log.Printf("error removing file: %v\n", err)
		}
		return "", err
	}

	return filePath, nil
}

type RenderDetails struct {
	data          string
	duration      time.Duration
	width, height int
}

func parsePath(path string) (RenderDetails, error) {
	strippedPath := path
	if strings.HasPrefix(path, "/") {
		strippedPath = strippedPath[1:]
	}

	if !strings.HasSuffix(path, ".png") {
		return RenderDetails{}, fmt.Errorf("invalid format %q, expected \".png\"", path)
	}
	strippedPath = strippedPath[0 : len(strippedPath)-4]

	parts := strings.Split(strippedPath, "-")
	if len(parts) < 3 {
		return RenderDetails{}, fmt.Errorf("invalid path format %q", path)
	}

	duration, err := time.ParseDuration(parts[1])
	if err != nil {
		return RenderDetails{}, fmt.Errorf("invalid duration %q", parts[1])
	}

	dimensions := strings.Split(parts[2], "x")
	if len(dimensions) != 2 {
		return RenderDetails{}, fmt.Errorf("invalid dimension format %q", path)
	}

	width, err := strconv.Atoi(dimensions[0])
	if err != nil {
		return RenderDetails{}, fmt.Errorf("invalid width %q", dimensions[0])
	}

	height, err := strconv.Atoi(dimensions[1])
	if err != nil {
		return RenderDetails{}, fmt.Errorf("invalid height %q", dimensions[1])
	}

	return RenderDetails{
		data:     parts[0],
		duration: duration,
		width:    width,
		height:   height,
	}, nil
}
