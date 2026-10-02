# Real-Time Chat Server — Architecture & Build Instructions

A standalone Go project to learn WebSockets and concurrency patterns, inspired by Yahoo Messenger. Built independently of the expense tracker project, but following the same layered architecture conventions.

## Project Goal

Learn Go's core concurrency primitives (goroutines, channels, the hub/broadcast pattern) by building a real-time chat server with presence, direct messages, and group rooms — the defining features of late-90s/2000s-era messenger apps.

## Scope (v1)

**In scope:**
- Buddy list with online/offline presence
- 1:1 direct messages
- Group chat rooms
- Message persistence (stored in Postgres, retrievable as history)
- Simple username-only "auth" (no password, no real security — out of scope by design)

**Explicitly deferred to later:**
- Typing indicators
- The "Buzz"/nudge feature
- Custom away messages
- Emoticons / rich message content
- Real authentication (passwords, JWT)
- Deployment (this project runs locally via Docker Compose; no live hosting needed)

## Tech Stack

- **Language:** Go
- **WebSocket library:** `github.com/gorilla/websocket`
- **Database:** PostgreSQL (via `pgx`, same driver as the expense tracker project)
- **Containerization:** Docker Compose (server + Postgres), following the same pattern as the expense tracker

## Why Presence Lives In Memory, Not the Database

This is the central architectural decision for the whole project. "Online/offline" is fundamentally about which WebSocket connections are currently open on this server process — it's transient, per-connection state that disappears the instant a connection drops. It should never require a database write on every connect/disconnect. Presence is tracked entirely in memory, inside the Hub (see below). The database only stores durable facts: users, rooms, room membership, and message history.

## Database Schema

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_members (
    room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    room_id INTEGER REFERENCES rooms(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((recipient_id IS NOT NULL AND room_id IS NULL) OR (recipient_id IS NULL AND room_id IS NOT NULL))
);
```

**Design notes:**
- One `messages` table handles both DMs and room messages, distinguished by whether `recipient_id` or `room_id` is populated. The `CHECK` constraint makes it structurally impossible to insert a message with both or neither set — this is enforced by Postgres itself, not just application logic.
- No `password` column on `users` — matches the "just a username" decision. Anyone can claim any username; there is no real authentication. This is acceptable for a local learning project and should not be treated as a pattern to reuse in anything with real users.
- `room_members` is a join table tracking durable membership (who belongs to which room) — separate from presence (who's currently online), which is not stored here.

## Core Architecture: The Hub Pattern

### `internal/chat/hub.go`

A single `Hub` struct running in its own goroutine, owning all shared connection state:

- `clients map[int]*Client` — currently connected users, keyed by user ID. Only the Hub's own goroutine ever reads or writes this map directly.
- `register chan *Client` — new connections are sent here when a client finishes connecting.
- `unregister chan *Client` — disconnections are sent here.
- `broadcast chan Message` — outgoing messages needing routing (to a DM recipient or to a room's members) arrive here.

The Hub's entire logic is a single loop:

```go
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// add to h.clients, mark online
		case client := <-h.unregister:
			// remove from h.clients, mark offline
		case message := <-h.broadcast:
			// persist to DB, then route to connected recipient(s)
		}
	}
}
```

**Why no mutex is needed here:** because only one goroutine (the Hub's own `Run` loop) ever touches `h.clients`, there's no concurrent access to protect against. This is Go's "share memory by communicating" philosophy in practice — instead of multiple goroutines locking a shared map, every goroutine that needs to affect the Hub's state sends a message on a channel, and the Hub serializes all of that state access by only ever processing one channel message at a time.

### `internal/chat/client.go`

Each connected WebSocket gets a `Client` struct with:

- **A read goroutine** — blocks on reading incoming messages from that specific WebSocket connection, forwarding parsed messages to the Hub's `broadcast` channel.
- **A write goroutine** — blocks reading from a per-client outbound channel (`send chan Message`), writing each message out to that client's WebSocket connection.

**Why two goroutines per client, not one:** a single WebSocket connection is not safe for concurrent writes from multiple goroutines. By giving each client exactly one dedicated writer goroutine, and having every other part of the system (the Hub) only ever push outbound messages onto that client's `send` channel (never writing to the socket directly), concurrent-write bugs become structurally impossible.

### `internal/chat/message.go`

Defines the `Message` struct and a `Store` interface for persistence — same pattern as `expense.Store` in the previous project: domain logic depends on an interface, not directly on Postgres.

### `internal/chat/postgres_store.go`

Implements message persistence: saving a new message, fetching DM history between two users, fetching a room's message history.

### How persistence and real-time delivery interact

When the Hub receives a message on its `broadcast` channel:
1. It writes the message to Postgres first (the database is the source of truth, and writing first guarantees message ordering even under concurrent load).
2. It then looks up whether the recipient (for a DM) or each room member (for a group message) is currently present in `h.clients`.
3. For each currently-connected recipient, it pushes the message onto that client's `send` channel for immediate delivery.
4. If a recipient is offline, the message simply isn't delivered in real time — it will be retrieved later from the database when they next fetch history (e.g., on reconnecting and loading recent messages).

## Proposed File Structure

```
chat-server/
├── cmd/
│   └── chat-server/
│       └── main.go              # entry point: config, DB pool, Hub startup, HTTP server
├── internal/
│   ├── chat/
│   │   ├── hub.go                # the Hub: central registry + router
│   │   ├── client.go             # per-connection read/write goroutines
│   │   ├── message.go            # Message struct, Store interface
│   │   └── postgres_store.go     # Postgres implementation of Store
│   ├── user/
│   │   └── user.go               # simple username-only lookup/creation (no passwords)
│   └── room/
│       └── room.go               # room creation, joining, listing members
├── docker-compose.yml
├── Dockerfile
└── go.mod
```

This mirrors the layered structure used in the expense tracker project (domain logic separated from transport/storage mechanics), for consistency.

## Testing Approach (No Frontend Required)

Since the goal is learning the backend concurrency patterns, not building a UI, test with either:
- A minimal single HTML file with a vanilla JS `WebSocket` client — fast to write, sufficient to manually verify DMs, rooms, and presence work across multiple browser tabs (simulating multiple users).
- Multiple terminal windows using a simple Go or `websocat`-based CLI client to simulate concurrent users, useful for testing concurrent load/race conditions specifically.

## Build Order (Suggested)

1. Schema + `internal/user` (create/look up a user by username, no auth logic needed)
2. `internal/chat/hub.go` + `internal/chat/client.go` — get the Hub/Client pattern working with a trivial "echo" broadcast (no persistence, no rooms yet) to validate the core concurrency model first
3. Wire in `internal/chat/postgres_store.go` for message persistence
4. Add `internal/room` and extend the Hub to route room messages (broadcast to all currently-connected members of a room) alongside 1:1 DMs
5. Add presence broadcasting (notify a user's contacts when they come online/offline) — this requires tracking "who cares about whom," a slightly more involved addition once the core routing works
6. Build the minimal HTML test client last, once the server-side behavior is confirmed correct via simpler testing (e.g., two terminal-based clients)

## Notes on Origin

This project was deliberately scoped as standalone and local-only, separate from the expense tracker project, specifically to focus on Go concurrency patterns (goroutines, channels) that the expense tracker's REST API never required. It reuses architectural conventions (layered `internal/` packages, Postgres via `pgx`, Docker Compose) from that earlier project for consistency, but shares no code with it.
