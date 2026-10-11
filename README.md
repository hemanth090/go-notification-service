# Notification Service (simple version)

A small Go web app that queues notifications and delivers them in the
background with automatic retries. Written to be **easy to read** — one
package, plain names, short functions, no interfaces.

## Run it

```bash
cd go-notification-service
go run .
```

Then open http://localhost:8080 and send a notification from the form.

## The files

| File             | What's inside                                              |
|------------------|------------------------------------------------------------|
| `main.go`        | Starts workers, starts the web server, shuts down cleanly  |
| `settings.go`    | Settings read from environment variables, and the queue    |
| `notification.go`| The notification type and the in-memory store              |
| `deliver.go`     | The three delivery methods: email, webhook, in-app         |
| `worker.go`      | Background workers that deliver and retry                  |
| `server.go`      | The HTTP handlers (`/api/notifications`, `/healthz`)       |
| `ui.go`          | Serves the web page (baked into the program)               |
| `index.html`     | The single-page web form                                   |

## How a notification flows

1. You `POST` a notification to `/api/notifications`.
2. The server checks it, saves it as `pending`, and puts it on the queue.
3. A background worker picks it up and calls `deliver()`.
4. If delivery fails, the worker waits and retries (up to 3 times).
5. The notification ends up `sent` or `failed`.

```
POST /api/notifications  ->  save  ->  jobs queue  ->  worker  ->  deliver()
                                   (pending)                        (sent/failed)
```

## The API

**Send one**
```bash
curl -X POST localhost:8080/api/notifications \
  -d '{"channel":"email","recipient":"ada@example.com","subject":"Hi","body":"yo"}'
```

**List all**
```bash
curl localhost:8080/api/notifications
```

**Get one**
```bash
curl localhost:8080/api/notifications/<id>
```

**Health check**
```bash
curl localhost:8080/healthz
```

## Settings

Set these environment variables before `go run .` if you want different values.

| Variable      | Default | Meaning                          |
|---------------|---------|----------------------------------|
| `ADDR`        | `:8080` | Port to listen on                |
| `WORKER_COUNT`| `4`     | How many background workers      |
| `QUEUE_SIZE`  | `256`   | How many jobs can wait in line   |
| `MAX_ATTEMPTS`| `3`     | How many times to retry delivery |

```bash
WORKER_COUNT=8 go run .
```

## Try the retry path

Send a webhook to an address that isn't listening — it will fail, then retry,
then give up and show up as `failed`:

```bash
curl -X POST localhost:8080/api/notifications \
  -d '{"channel":"webhook","recipient":"http://localhost:9999/dead"}'
```
