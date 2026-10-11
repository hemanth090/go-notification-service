// A small notification service.
//
// What it does: you send a notification over HTTP, it gets saved and put on a
// queue, and background workers deliver it (email, webhook, or in-app) with
// automatic retries.
//
// The code is split into a few small files, all in package main:
//
//	settings.go     - app settings read from the environment
//	notification.go - the notification type and the in-memory store
//	deliver.go      - the three delivery methods
//	worker.go       - background workers that deliver and retry
//	server.go       - the HTTP handlers
//	ui.go           - serves the web page
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Start the workers that deliver notifications in the background.
	startWorkers(workerCount, jobs)

	// Set up the web routes.
	router := http.NewServeMux()
	router.HandleFunc("POST /api/notifications", createHandler)
	router.HandleFunc("GET /api/notifications", listHandler)
	router.HandleFunc("GET /api/notifications/{id}", getHandler)
	router.HandleFunc("GET /healthz", healthHandler)
	router.Handle("GET /", http.FileServer(http.FS(uiFiles)))

	server := &http.Server{Addr: listenAddress, Handler: router}

	// Stop the whole program when the user presses Ctrl+C.
	shutdownContext, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListening()

	// Run the server in the background so we can wait for Ctrl+C below.
	go func() {
		slog.Info("listening", "addr", listenAddress)
		serveError := server.ListenAndServe()
		if serveError != nil && serveError != http.ErrServerClosed {
			slog.Error("server error", "err", serveError)
			os.Exit(1)
		}
	}()

	<-shutdownContext.Done() // blocks until Ctrl+C
	slog.Info("shutting down")

	// 1. Stop accepting new requests.
	timeoutContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	server.Shutdown(timeoutContext)

	// 2. Let the workers finish whatever is already in the queue.
	close(jobs)
	waitForWorkers(shutdownTimeout)

	slog.Info("bye")
}
