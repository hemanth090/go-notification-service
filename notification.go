package main

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// The channels we know how to deliver to.
const (
	channelEmail   = "email"
	channelWebhook = "webhook"
	channelInApp   = "inapp"
)

// The states a notification can be in.
const (
	statusPending = "pending"
	statusSent    = "sent"
	statusFailed  = "failed"
)

// notification is one message we need to deliver.
type notification struct {
	ID        string    `json:"id"`
	Channel   string    `json:"channel"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"lastError,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// store keeps notifications in memory. It is safe to use from many goroutines
// at once because every method locks the lock first.
type store struct {
	lock          sync.RWMutex
	notifications map[string]notification
}

func newStore() *store {
	return &store{notifications: make(map[string]notification)}
}

// save adds a new notification, or updates one that already exists.
func (dataStore *store) save(item notification) {
	dataStore.lock.Lock()
	defer dataStore.lock.Unlock()

	now := time.Now()
	if existing, found := dataStore.notifications[item.ID]; found {
		item.CreatedAt = existing.CreatedAt // keep the original created time
	} else {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	dataStore.notifications[item.ID] = item
}

// get returns one notification by id. The bool is false if it doesn't exist.
func (dataStore *store) get(notificationID string) (notification, bool) {
	dataStore.lock.RLock()
	defer dataStore.lock.RUnlock()
	item, found := dataStore.notifications[notificationID]
	return item, found
}

// list returns all notifications, newest first.
func (dataStore *store) list() []notification {
	dataStore.lock.RLock()
	defer dataStore.lock.RUnlock()

	allNotifications := make([]notification, 0, len(dataStore.notifications))
	for _, item := range dataStore.notifications {
		allNotifications = append(allNotifications, item)
	}
	sort.Slice(allNotifications, func(first, second int) bool {
		return allNotifications[first].CreatedAt.After(allNotifications[second].CreatedAt)
	})
	return allNotifications
}

// newNotificationID makes a short random id. We use crypto/rand so ids are
// hard to guess.
func newNotificationID() string {
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}
