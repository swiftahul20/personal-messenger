# Personal Messenger

A local real-time chat server written in Go. It uses WebSockets and a channel-based Hub for direct messages, group rooms, and in-memory online presence. PostgreSQL stores users, buddy relationships, room membership, and message history.

This is a learning project. Usernames are not passwords: anyone can connect as a known user ID. Do not expose this server to an untrusted network or use it for real accounts.

## Features

- Create or retrieve a username-only user
- Add buddies and see their current online status
- Send real-time 1:1 direct messages to buddies
- Insert Unicode emoji mapped from classic Yahoo emoticon codes
- Create rooms, join rooms, and exchange messages with room members
- Retrieve persisted direct-message and room history
- One Hub goroutine owns online client state; each WebSocket client has separate read and write loops

Typing indicators are not stored. Away messages, rich content, and real authentication are not included.

## Requirements

- Go 1.23 or newer
- Docker Desktop with Docker Compose
- Docker Hub access the first time images are pulled
- Node.js 22.12 or newer and npm (only needed to run the React frontend outside Docker)

## Frontend Setup

The React + TypeScript chat UI is in `web/`. To start the database, Go server, and frontend together with live frontend source updates, run:

```powershell
docker compose up --build
```

Open `http://localhost:5173`. In Docker, Vite proxies `/api` and `/ws` to the Go server by its Compose service name.

To run the frontend directly on your host instead, start the Go server first, then in another PowerShell terminal run:

```powershell
Set-Location web
npm ci
npm run dev
```

Open the Vite URL printed in the terminal, usually `http://localhost:5173`. The development server proxies `/api` and `/ws` to the Go server on port `8080`.

To verify or lint the frontend:

```powershell
Set-Location web
npm run build
npm run lint
```

## Run with Docker Compose

From the project directory:

```powershell
docker compose up --build
```

The server is available at `http://localhost:8080`; PostgreSQL is exposed on port `5432`. The server applies `internal/data/schema.sql` at startup. Stop the services with `Ctrl+C`; use `docker compose down` to stop and remove the containers. The named `postgres_data` volume preserves database data. To delete that data too, run `docker compose down -v`.

To run only PostgreSQL in Docker and the Go server on the host, start the database with `docker compose up -d db`, then in PowerShell run:

```powershell
$env:DATABASE_URL = 'postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable'
go run ./cmd/chat-server
```

## API

All request and response bodies use JSON.

| Method | Endpoint                                           | Purpose                                    |
| ------ | -------------------------------------------------- | ------------------------------------------ |
| `GET`  | `/api/healthz`                                     | Health check                               |
| `POST` | `/api/users`                                       | Create or retrieve a user                  |
| `GET`  | `/api/users/{userID}/buddies`                      | List buddies and their online status       |
| `POST` | `/api/users/{userID}/buddies`                      | Add a buddy by username                    |
| `GET`  | `/api/users/{userID}/rooms`                        | List rooms the user joined                 |
| `POST` | `/api/rooms`                                       | Create a room; creator joins automatically |
| `POST` | `/api/rooms/{roomID}/join`                         | Join a room                                |
| `GET`  | `/api/messages/dm?user_id={id}&with={buddyID}`     | Fetch direct-message history               |
| `GET`  | `/api/messages/room?user_id={id}&room_id={roomID}` | Fetch room history                         |
| `GET`  | `/ws?user_id={id}`                                 | Open a WebSocket connection                |

History endpoints accept an optional `limit` query parameter (default 50, maximum 200). Users must be buddies to exchange or fetch direct messages. Users must be room members to send or fetch room messages.

### Create users and a buddy relationship

Run each command in PowerShell. The response includes the numeric user `id`.

```powershell
$alice = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/users -ContentType 'application/json' -Body '{"username":"alice"}'
$bob = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/users -ContentType 'application/json' -Body '{"username":"bob"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/users/$($alice.id)/buddies" -ContentType 'application/json' -Body '{"username":"bob"}'
```

Adding a buddy is reciprocal. Usernames must contain 3-50 letters, numbers, or underscores.

### Create and join a room

```powershell
$roomBody = @{ user_id = [int]$alice.id; name = 'after-hours' } | ConvertTo-Json -Compress
$room = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/rooms -ContentType 'application/json' -Body $roomBody
$joinBody = @{ user_id = [int]$bob.id } | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/rooms/$($room.id)/join" -ContentType 'application/json' -Body $joinBody
```

### Try WebSocket messaging

Open two terminals and connect as the users created above. Keep each process running in its own terminal:

```powershell
npx --yes wscat -c "ws://localhost:8080/ws?user_id=$($alice.id)"
```

```powershell
npx --yes wscat -c "ws://localhost:8080/ws?user_id=$($bob.id)"
```

At Alice's `wscat` prompt, send a direct message (replace `2` with Bob's actual ID):

```json
{ "type": "dm", "recipient_id": 2, "content": "Hi Bob" }
```

Send a room message (replace `1` with the room's actual ID):

```json
{ "type": "room", "room_id": 1, "content": "Hello room" }
```

Each connected recipient receives a JSON event with `"type":"message"`. Presence events have `"type":"presence"`; invalid messages and delivery failures return `"type":"error"`.

To show that you are typing, send `{ "type": "typing", "recipient_id": 2 }` or `{ "type": "typing", "room_id": 1 }`. The server does not store it and sends no reply. The buddy, or the other room members, receive `{ "type": "typing", "user_id": 1 }` (with `room_id` for rooms). Resend about every two seconds while typing. Receivers should treat a gap of four seconds as the user having stopped.

### Fetch message history

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/dm?user_id=$($alice.id)&with=$($bob.id)"
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/room?user_id=$($alice.id)&room_id=$($room.id)"
```

## Web client

A React, TypeScript, and Tailwind client lives in `web/`. Design direction is in `DESIGN.md`. It needs Node 20.19 or newer (22.13 or newer is recommended).

```powershell
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. The dev server proxies `/api` and `/ws` to the Go server on port 8080, so start the server first.

Sign in with a username in each window. Choose **Side by side** to run two signed-in users next to each other: a message sent in one window appears in the other as soon as the server delivers it, and the other window shows "is typing..." while the sender types. Use the emoji button to insert a Unicode equivalent of a classic Yahoo emoticon at the cursor. Both users need to be buddies, so add one from the other's window.

## Run checks

```powershell
go test ./...
go test -race ./...
go build ./cmd/chat-server
```

## Project layout

```text
cmd/chat-server/       Server entry point
web/                   React client
DESIGN.md              Design direction for the client
web/                   React + TypeScript frontend scaffold (no UI yet)
internal/api/          HTTP and WebSocket endpoints
internal/chat/         Hub, clients, and chat message contracts
internal/data/         PostgreSQL store and embedded schema
internal/room/         Room domain types and validation
internal/user/         User domain types and validation
docker-compose.yml     Local PostgreSQL and server
Dockerfile             Multi-stage Go build
instructions.md        Original architecture and build notes
```
