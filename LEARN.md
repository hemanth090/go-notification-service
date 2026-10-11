# LEARN.md — Build this yourself, from zero

This guide walks you through building the notification backend **by hand**.
No copy-paste magic. You type each piece, run it, see it work, then add the next.

The rules:
1. **Every step compiles and runs.** You always have something working.
2. **Plain names only.** No `mu`, `n`, `res`. Full words.
3. **Short explanations.** What a thing is, why we use it, done.

The finished code already lives in this folder. This guide explains how you'd
arrive at it, step by step.

---

## The 6 ideas this whole project uses

Learn these six and the code stops looking scary:

1. **A web server** — a program that waits for requests on a port and replies.
   Go's `net/http` gives you one.
2. **A struct** — a custom box that groups related fields. Like a blank form:
   "Recipient: ___, Subject: ___, Body: ___".
3. **A map** — a dictionary. `map[string]notification` = "look up a
   notification by its id string".
4. **A goroutine** — code that runs at the same time as everything else.
   Write `go doWork()` and it runs in the background. Go makes these cheap.
5. **A channel** — a typed pipe between goroutines. One side sends, the other
   receives. `chan notification` is a pipe that carries notifications.
6. **A mutex (a "lock")** — a single-key bathroom. Only one goroutine holds the
   key at a time, so two workers can't write to the map at the same instant.

Bonus idea: **Go functions return errors instead of throwing.** When something
can fail, the function returns an `error` as its last value, and you check it:
```go
result, err := somethingThatMightFail()
if err != nil {
    // handle it
}
```

---

## What we're building

You send a notification over HTTP. It gets saved, put on a queue, and
background workers deliver it (email/webhook/in-app) with automatic retries.

```
POST /api/notifications
        │
        ▼
  save it as "pending"          ← server.go
        │
        ▼
  jobs channel (the queue)
        │
        ▼
  worker picks it up            ← worker.go
        │
        ▼
  deliver() sends it            ← deliver.go
        │
        ▼
  mark it "sent" or "failed"    ← notification.go (the store)
```

---

## How to follow along

Make a scratch folder for the early experiments:
```bash
mkdir -p /tmp/go-practice && cd /tmp/go-practice
```
Steps 1–3 are **standalone experiments**. Steps 4–9 assemble the **real app**.

---

## Step 1 — The notification (your first struct)

A **struct** is just a box with named fields. Our box is a notification.

Type this in `/tmp/go-practice/main.go` and run it:

```go
package main

import "fmt"

// notification is one message we need to deliver.
type notification struct {
    Recipient string
    Subject   string
    Body      string
}

func main() {
    first := notification{
        Recipient: "ada@example.com",
        Subject:   "Welcome!",
        Body:      "Hi Ada, glad you're here.",
    }

    fmt.Println("Sending to:", first.Recipient)
    fmt.Println("Subject:", first.Subject)
    fmt.Println("Body:", first.Body)
}
```

```bash
go run .
```

**What you learned**
- `package main` + `func main()` — every runnable Go program starts here.
- `type notification struct { ... }` — you invented a new type.
- `notification{ Recipient: "..." }` — you made one and filled its fields.
- `first.Recipient` — the dot reaches into a field.

---

## Step 2 — A place to keep them (the store, a map, and a lock)

Notifications vanish unless we keep them somewhere. A **map** is perfect:
id → notification.

Replace `/tmp/go-practice/main.go` with this and run it:

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

type notification struct {
    ID        string
    Recipient string
    Status    string
    CreatedAt time.Time
}

// store holds notifications in memory.
type store struct {
    lock          sync.RWMutex
    notifications map[string]notification
}

// newStore makes an empty store, ready to use.
func newStore() *store {
    return &store{notifications: make(map[string]notification)}
}

// save puts a notification into the store (locked so it's safe).
func (dataStore *store) save(item notification) {
    dataStore.lock.Lock()         // take the key
    defer dataStore.lock.Unlock() // give it back when this function ends

    item.CreatedAt = time.Now()
    dataStore.notifications[item.ID] = item
}

func main() {
    database := newStore()

    database.save(notification{ID: "aaa", Recipient: "ada@example.com", Status: "pending"})
    database.save(notification{ID: "bbb", Recipient: "bob@example.com", Status: "sent"})

    fmt.Println("Total saved:", len(database.notifications))
    fmt.Println("id aaa goes to:", database.notifications["aaa"].Recipient)
}
```

```bash
go run .
```

**What you learned**
- `map[string]notification` — a dictionary keyed by id.
- `func (dataStore *store) save(...)` — a **method**. `save` is something a
  `store` can *do*. The `*store` means "the real store, not a copy", so the
  lock and map are shared, not duplicated.
- `lock.Lock()` / `defer lock.Unlock()` — grab the key; `defer` guarantees we
  hand it back when the function exits, even on error.
- `RWMutex` — a lock that lets many readers OR one writer (readers don't block
  each other).

---

## Step 3 — Read settings from the environment

Hard-coding `:8080` is fine until you want to change it. Instead read an
**environment variable**, with a fallback default.

```go
func readTextSetting(name string, fallback string) string {
    value := os.Getenv(name)
    if value != "" {
        return value
    }
    return fallback
}

func readNumberSetting(name string, fallback int) int {
    value := os.Getenv(name)
    if value != "" {
        number, conversionError := strconv.Atoi(value) // text -> number
        if conversionError == nil {
            return number
        }
    }
    return fallback
---

## Step 4 — Talk over HTTP (handlers + JSON)

A **handler** is a function that receives a request and writes a response:

```go
func myHandler(responseWriter http.ResponseWriter, request *http.Request)
```

- `responseWriter` — you write your reply into this.
- `request` — the incoming request (body, path, etc.).

Two helpers every handler uses:

```go
func writeJSON(responseWriter http.ResponseWriter, status int, data any) {
    responseWriter.Header().Set("Content-Type", "application/json")
    responseWriter.WriteHeader(status)
    json.NewEncoder(responseWriter).Encode(data) // struct -> JSON text
}

func createHandler(responseWriter http.ResponseWriter, request *http.Request) {
    var item notification
    decodeError := json.NewDecoder(request.Body).Decode(&item) // JSON -> struct
    if decodeError != nil {
        writeJSON(responseWriter, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
        return
    }

    item.ID = newNotificationID()
    item.Status = "pending"
    database.save(item)

    writeJSON(responseWriter, http.StatusAccepted, item)
}
```

Start a server in `main`:

```go
router := http.NewServeMux()
router.HandleFunc("POST /api/notifications", createHandler)

server := &http.Server{Addr: listenAddress, Handler: router}
serveError := server.ListenAndServe()
if serveError != nil && serveError != http.ErrServerClosed {
    fmt.Println("server error:", serveError)
}
```

Run it, then in another terminal:
```bash
curl -X POST localhost:8080/api/notifications \
  -d '{"recipient":"ada@example.com","subject":"Hi"}'
```

**What you learned**
- `http.NewServeMux()` — a router. `HandleFunc` maps "this path → this handler".
- `Decode(&item)` turns request JSON into a struct; `Encode` does the reverse.
- `http.StatusAccepted` (202) and friends are just named numbers.

---

## Step 5 — The queue (a channel)

Right now `createHandler` saves and returns. We want delivery to happen in the
**background**, so the request returns instantly. The tool for "hand work to
another part of the program" is a **channel**.

Make one buffered channel (it can hold many items without a reader waiting):
```go
var jobs = make(chan notification, 256)
```

**Send** onto it (in `createHandler`, after saving):
```go
jobs <- item // put it on the queue
```

**Receive** from it (this blocks until an item arrives):
```go
item := <-jobs
```

Or loop forever, taking one at a time (this is what a worker does):
```go
for item := range jobs {
    // deliver this one
}
```

**What you learned**
- `make(chan notification, 256)` — a pipe that carries notifications; the `256`
  is a waiting room, so sending doesn't block until the room is full.
- `<-` is the direction: `jobs <- item` sends, `item := <-jobs` receives.


}
```

Use them like:
```go
listenAddress := readTextSetting("ADDR", ":8080")
workerCount := readNumberSetting("WORKER_COUNT", 4)
```

Run with a custom value:
```bash
ADDR=:9000 WORKER_COUNT=8 go run .
```

**What you learned**
- `os.Getenv("ADDR")` reads an environment variable (empty if not set).
- `strconv.Atoi` turns `"4"` into the number `4`. It can fail, so it returns an
  error you check.


---

## Step 6 — Workers (goroutines) + retries

A **goroutine** is a piece of code running on its own, in the background. We
want several workers all pulling from the same `jobs` channel.

```go
var workerGroup sync.WaitGroup // counts how many workers are still running

func startWorkers(totalWorkers int, queue <-chan notification) {
    for workerNumber := 0; workerNumber < totalWorkers; workerNumber++ {
        workerGroup.Add(1) // one more worker to wait for
        go func(workerID int) {
            defer workerGroup.Done() // "I'm finished" when this returns
            for item := range queue {
                deliverWithRetry(item)
            }
        }(workerNumber)
    }
}
```

Read that inner `go func(...)` as: "start a background worker that keeps taking
items off the queue and delivering them until the queue closes."

**The retry loop** — try to deliver; if it fails, wait and try again:

```go
func deliverWithRetry(item notification) {
    var lastError error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        item.Attempts = attempt
        lastError = deliver(item)          // try to send it
        if lastError == nil {              // no error = success!
            item.Status = "sent"
            database.save(item)
            return
        }
        // failed: wait a bit, then loop tries again
        time.Sleep(backoff(attempt))
    }
    item.Status = "failed"                 // ran out of tries
    item.LastError = lastError.Error()
    database.save(item)
}

// backoff doubles each time: 200ms, 400ms, 800ms, ...
func backoff(attempt int) time.Duration {
    return 200 * time.Millisecond << (attempt - 1)
}
```

**What you learned**
- `go func(){}()` — run the function in the background (a goroutine).
- `sync.WaitGroup` — `Add(1)` when starting, `Done()` when finished, `Wait()`
  blocks until the count hits zero. Used at shutdown.
- `<<` is a left-shift; `<< 1` doubles a number, `<< 2` quadruples it.

---

## Step 7 — Delivery (the three channels)

`deliver` picks the right method based on the channel:

```go
func deliver(item notification) error {
    switch item.Channel {
    case "email":
        return sendEmail(item)
    case "webhook":
        return sendWebhook(item)
    case "inapp":
        return sendInApp(item)
    default:
        return fmt.Errorf("unknown channel: %s", item.Channel)
    }
}
```

`sendEmail` and `sendInApp` are pretend (they just log). `sendWebhook` is
**real** — it posts JSON to a URL:

```go
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
```

**What you learned**
- `switch` picks one path based on a value.
- `http.NewRequest` + `client.Do` = actually make an HTTP call.
- Notice the pattern: every step that can fail returns an `error`, checked
  immediately. That's the Go way.



---

## Step 8 — Graceful shutdown

When you press Ctrl+C, the OS sends a signal. We want to: stop taking new
requests, then let workers finish what's already queued, then exit.

```go
// Gives us a context that is cancelled when Ctrl+C (or SIGTERM) arrives.
shutdownContext, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stopListening()

<-shutdownContext.Done() // blocks here until Ctrl+C
slog.Info("shutting down")

// 1. Stop accepting new requests.
timeoutContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancelShutdown()
server.Shutdown(timeoutContext)

// 2. Tell workers "no more items are coming", then wait for them to finish.
close(jobs)
waitForWorkers(shutdownTimeout)
```

`waitForWorkers` waits for the WaitGroup, but gives up after a timeout so
shutdown can never hang forever:
```go
func waitForWorkers(timeout time.Duration) {
    workersFinished := make(chan struct{})
    go func() {
        workerGroup.Wait()
        close(workersFinished)
    }()
    select {
    case <-workersFinished:
    case <-time.After(timeout):
        slog.Warn("workers did not finish in time")
    }
}
```

**What you learned**
- `signal.NotifyContext` turns a Ctrl+C into a cancellable context.
- `<-shutdownContext.Done()` blocks until that happens.
- `close(jobs)` closes the channel; a `for item := range jobs` loop then ends
  naturally. That's the clean way to tell workers "wrap up".
- `select` waits on whichever channel is ready first (finished OR timeout).

---

## Step 9 — The web page (embed)

The HTML page lives in `index.html`. We bake it into the program so there's no
extra file to carry around:

```go
import "embed"

//go:embed index.html
var uiFiles embed.FS
```

Then serve it:
```go
router.Handle("GET /", http.FileServer(http.FS(uiFiles)))
```

The page uses plain JavaScript `fetch()` to call your API: `send()` POSTs a
notification, and `load()` GETs the list every 2 seconds to refresh it.

**What you learned**
- `//go:embed index.html` — the comment is a real instruction; it copies the
  file into the compiled binary.
- The browser talks to your Go server using the same HTTP you already built.

---

## How it all fits together (the real files)

You've now seen every piece. In the real project they're split for clarity:

| File             | Holds                                                        |
|------------------|--------------------------------------------------------------|
| `main.go`        | Wiring: start workers, start server, shutdown (Steps 1,8)    |
| `settings.go`    | Environment settings + the `jobs` channel (Steps 3,5)        |
| `notification.go`| The `notification` struct + the `store` (Steps 1,2)          |
| `deliver.go`     | `deliver` + `sendEmail`/`sendWebhook`/`sendInApp` (Step 7)   |
| `worker.go`      | `startWorkers`, `deliverWithRetry`, `backoff` (Step 6)       |
| `server.go`      | The HTTP handlers + `writeJSON`/`writeError` (Step 4)        |
| `ui.go`          | The `//go:embed` for the web page (Step 9)                   |
| `index.html`     | The web form (Step 9)                                        |

The flow, end to end:
```
browser (index.html) --POST--> createHandler (server.go)
                                   │ save "pending" (notification.go)
                                   ▼
                              jobs <- item   (channel)
                                   │
                                   ▼
                          worker takes it (worker.go)
                                   │ deliverWithRetry
                                   ▼
                          deliver() (deliver.go) → sent / failed
```

---

## Run the real thing

```bash
cd go-notification-service
go run .
```
Open http://localhost:8080 and send a notification.

See the retry path (webhook to a dead URL → fails 3× → "failed"):
```bash
curl -X POST localhost:8080/api/notifications \
  -d '{"channel":"webhook","recipient":"http://localhost:9999/dead"}'
```

---

## Your next practice ideas

1. Add a `DELETE /api/notifications/{id}` handler + a `delete` method on the
   store.
2. Make `sendEmail` actually print the full email body (not just the subject).
3. Add a `Priority` field; deliver high-priority items first.
4. Change `maxAttempts` via the environment and watch retries change.

You now understand every moving part. Build on top of it.
