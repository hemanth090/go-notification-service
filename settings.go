package main

import (
	"os"
	"strconv"
	"time"
)

// These are the app's settings. Each one reads from an environment variable,
// or falls back to a default when the variable isn't set.
var (
	listenAddress   = readTextSetting("ADDR", ":8080")
	workerCount     = readNumberSetting("WORKER_COUNT", 4)
	queueSize       = readNumberSetting("QUEUE_SIZE", 256)
	maxAttempts     = readNumberSetting("MAX_ATTEMPTS", 3)
	shutdownTimeout = 10 * time.Second
)

// jobs is the queue of notifications waiting to be delivered.
var jobs = make(chan notification, queueSize)

// database keeps every notification in memory so we can look them up later.
var database = newStore()

// readTextSetting reads a text setting from the environment.
func readTextSetting(name string, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}

// readNumberSetting reads a number setting from the environment.
func readNumberSetting(name string, fallback int) int {
	value := os.Getenv(name)
	if value != "" {
		number, conversionError := strconv.Atoi(value)
		if conversionError == nil {
			return number
		}
	}
	return fallback
}
