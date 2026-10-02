# Personal Messenger

A local real-time chat server written in Go. It uses WebSockets and a channel-based Hub for direct messages, group rooms, and in-memory online presence. PostgreSQL stores users, buddy relationships, room membership, and message history.

This is a learning project. Usernames are not passwords: anyone can connect as a known user ID. Do not expose this server to an untrusted network or use it for real accounts.

## Features

- Create or retrieve a username-only user
- Add buddies and see their current online status
- Send real-time 1:1 direct messages to buddies
- Create rooms, join rooms, and exchange messages with room members
- Retrieve persisted direct-message and room history
- One Hub goroutine owns online client state; each WebSocket client has separate read and write loops

Typing indicators, away messages, rich content, and real authentication are not included.

## Requirements

- Go 1.23 or newer
- Docker Desktop with Docker Compose
- Docker Hub access the first time images are pulled

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

### Fetch message history

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/dm?user_id=$($alice.id)&with=$($bob.id)"
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/room?user_id=$($alice.id)&room_id=$($room.id)"
```

## Run checks

```powershell
go test ./...
go test -race ./...
go build ./cmd/chat-server
```

## Project layout

```text
cmd/chat-server/       Server entry point
internal/api/          HTTP and WebSocket endpoints
internal/chat/         Hub, clients, and chat message contracts
internal/data/         PostgreSQL store and embedded schema
internal/room/         Room domain types and validation
internal/user/         User domain types and validation
docker-compose.yml     Local PostgreSQL and server
Dockerfile             Multi-stage Go build
instructions.md        Original architecture and build notes
```
