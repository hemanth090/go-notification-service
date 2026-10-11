package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// deliver sends a notification using the right method for its channel.
// It returns an error if delivery failed, so the worker can retry.
func deliver(item notification) error {
	switch item.Channel {
	case channelEmail:
		return sendEmail(item)
	case channelWebhook:
		return sendWebhook(item)
	case channelInApp:
		return sendInApp(item)
	default:
		return fmt.Errorf("unknown channel: %s", item.Channel)
	}
}

// sendEmail pretends to send an email. Swap this for a real provider later
// (for example Amazon SES or SendGrid).
func sendEmail(item notification) error {
	slog.Info("sending email", "to", item.Recipient, "subject", item.Subject)
	return nil
}

// sendWebhook really posts the notification as JSON to the recipient URL.
func sendWebhook(item notification) error {
	jsonBody, marshalError := json.Marshal(item)
	if marshalError != nil {
		return marshalError
	}

	request, requestError := http.NewRequest("POST", item.Recipient, bytes.NewReader(jsonBody))
	if requestError != nil {
		return requestError
	}
	request.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 5 * time.Second}
	response, responseError := httpClient.Do(request)
	if responseError != nil {
		return responseError
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", response.StatusCode)
	}
	return nil
}

// sendInApp pretends to store an in-app notification. Swap this for a real
// notification feed later.
func sendInApp(item notification) error {
	slog.Info("storing in-app notification", "user", item.Recipient)
	return nil
}
