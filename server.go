package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// createHandler accepts a new notification and puts it on the queue.
func createHandler(responseWriter http.ResponseWriter, request *http.Request) {
	var item notification
	decodeError := json.NewDecoder(request.Body).Decode(&item)
	if decodeError != nil {
		writeError(responseWriter, http.StatusBadRequest, "bad JSON")
		return
	}

	// Clean up the input a little.
	item.Channel = strings.ToLower(strings.TrimSpace(item.Channel))
	item.Recipient = strings.TrimSpace(item.Recipient)

	if !isValidChannel(item.Channel) {
		writeError(responseWriter, http.StatusBadRequest, "channel must be email, webhook, or inapp")
		return
	}
	if item.Recipient == "" {
		writeError(responseWriter, http.StatusBadRequest, "recipient is required")
		return
	}

	// Give it an id and a starting status, then save it.
	item.ID = newNotificationID()
	item.Status = statusPending
	database.save(item)

	// Put it on the queue. Using the request context means a shutdown or a
	// full queue won't hang this request forever.
	select {
	case jobs <- item:
	case <-request.Context().Done():
		writeError(responseWriter, http.StatusServiceUnavailable, "server is shutting down")
		return
	}

	writeJSON(responseWriter, http.StatusAccepted, item)
}

// listHandler returns every notification.
func listHandler(responseWriter http.ResponseWriter, request *http.Request) {
	writeJSON(responseWriter, http.StatusOK, database.list())
}

// getHandler returns one notification by id.
func getHandler(responseWriter http.ResponseWriter, request *http.Request) {
	item, found := database.get(request.PathValue("id"))
	if !found {
		writeError(responseWriter, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(responseWriter, http.StatusOK, item)
}

// healthHandler is a simple check that the server is alive.
func healthHandler(responseWriter http.ResponseWriter, request *http.Request) {
	writeJSON(responseWriter, http.StatusOK, map[string]string{"status": "ok"})
}

func isValidChannel(channel string) bool {
	return channel == channelEmail || channel == channelWebhook || channel == channelInApp
}

func writeJSON(responseWriter http.ResponseWriter, status int, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	json.NewEncoder(responseWriter).Encode(data)
}

func writeError(responseWriter http.ResponseWriter, status int, message string) {
	writeJSON(responseWriter, status, map[string]string{"error": message})
}
