# Personal Messenger

A local real-time chat server written in Go. It uses WebSockets and a channel-based Hub for direct messages, group rooms, and in-memory online presence. PostgreSQL stores users, buddy relationships, room membership, and message history.

This is a learning project. Usernames are not passwords: anyone can connect as a known user ID. Do not expose this server to an untrusted network or use it for real accounts.

## Features

- Create or retrieve a username-only user
- Add buddies and see their current online status with a green online indicator
- Send real-time 1:1 direct messages to buddies
- Show actual sender usernames in room messages
- Reconnect WebSocket sessions automatically after a disconnect
- Insert Unicode emoji mapped from classic Yahoo emoticon codes
- See whether messages are saved or queued to a live recipient
- Load older messages in pages while preserving scroll position
- Create rooms, find rooms by name, join them, and exchange messages with room members
- See who is in a room and who is online
- Retrieve persisted direct-message and room history
- Chat with CyberKid00, the always-online bot buddy available to every user
- Use CyberKid00 commands: `/help`, `/8ball`, `/roll [NdM]`, and `/time`
- Send a BUZZ to an online buddy from a direct-message window
- One Hub goroutine owns online client state; each WebSocket client has separate read and write loops

Typing indicators and BUZZ events are not stored. Rich content and real authentication are not included.

## CyberKid00

CyberKid00 is added to every user's buddy list and appears online. The username is reserved. Direct messages to the bot are saved in the normal conversation history.

Slash commands are handled by the Go server without calling Gemini:

| Command             | What it does                                                                |
| ------------------- | --------------------------------------------------------------------------- |
| `/help`             | Lists the commands and mentions BUZZ and Gemini tools                       |
| `/8ball [question]` | Replies with a random answer to a yes/no question                           |
| `/roll [NdM]`       | Rolls dice, defaulting to `1d6`; supports 1-100 dice with 2-1,000,000 sides |
| `/time`             | Shows the bot server's current time                                         |

Other messages use Gemini when `GEMINI_API_KEY` is configured. Gemini replies stream into the direct message as they are generated; the completed reply is saved to normal conversation history. Stream chunks are temporary and are not stored as separate messages. If the key is missing or Gemini cannot respond, CyberKid00 stays online but its presence changes to **Away from keyboard (AFK)**. The bot does not send a canned reply for these failures. A successful Gemini response restores its presence to **Online**. Non-command messages are limited by `BOT_USER_RPM` and `BOT_DAILY_CAP`.

With Gemini enabled, ask CyberKid00 to start a trivia game, then send your answer in the same direct message. You can also ask for current weather in a city or place. Weather lookups send the requested place name to Open-Meteo and include its name in the reply. Results use Celsius and kilometers per hour. Avoid sending private or sensitive location details.

Gemini can call only the trivia and weather tools. The server validates tool names and arguments and stops after three tool calls for a reply. Trivia state is kept per user in memory and expires after 15 minutes.

To configure the bot for Docker Compose, create a root-level `.env` file before starting the services. `.env` is ignored by Git. Add your Gemini API key there:

```dotenv
BOT_ENABLED=true
BOT_USERNAME=CyberKid00
BOT_USER_RPM=5
BOT_DAILY_CAP=100
GEMINI_MODEL=gemini-3.8-flash
GEMINI_API_KEY=your-gemini-api-key
```

Do not commit the `.env` file or put the key in the Docker image. After changing these settings, restart the server container with `docker compose up -d --force-recreate server`.

In the web client, open a direct message with CyberKid00 to use the commands. To BUZZ, open a direct message with an online buddy and select **Buzz** in the conversation header. The recipient's messenger window shakes briefly; reduced-motion settings suppress the animation. BUZZ is available for online buddies, not rooms or the bot.

## Requirements

- Docker Desktop with Docker Compose, and Docker Hub access the first time images are pulled
- Free ports `5173` (web), `8080` (API), and `5432` (PostgreSQL). Stop any local PostgreSQL first.
- Go 1.23 or newer and Node.js 22.12 or newer are only needed to run the server or web client outside Docker, or to run the checks.

The commands in this file use PowerShell syntax. On macOS or Linux, run `docker compose` the same way and use `curl` instead of `Invoke-RestMethod`.

## Quick start

```powershell
git clone https://github.com/swiftahul20/personal-messenger.git
Set-Location personal-messenger
docker compose up --build
```

The first run downloads images, compiles the Go server, and installs the web dependencies, which can take a few minutes. The web app is ready when the log prints Vite's `Local: http://localhost:5173/` line.

1. Open `http://localhost:5173`.
2. Sign in as `alice`. A username is all it takes; there is no password.
3. Choose **Side by side**, then sign in as `bob` in the second window.
4. In Alice's window, type `bob` under **Add buddy**, select him, and send a message. It appears in Bob's window.

The API is at `http://localhost:8080`, and PostgreSQL is exposed on port `5432`. The server applies `internal/data/schema.sql` at startup. Press `Ctrl+C` to stop the services, or run `docker compose down` to stop and remove the containers. The named `postgres_data` volume keeps your data; `docker compose down -v` deletes it too.

### Troubleshooting

- **A port is already in use:** stop whatever is using `5173`, `8080`, or `5432`, then run `docker compose up` again.
- **Edits under `web/` do not show up:** Docker bind mounts can miss file changes on Windows. Run `docker compose restart web`.
- **Images fail to download:** Docker cannot reach Docker Hub. Check Docker Desktop's network and proxy settings.
- **Docker fails with read-only or input/output errors:** the disk is probably full. Free space, then restart Docker Desktop.

## Run the services separately

Use this to work on the Go server or the web client outside Docker. Start only the database in Docker, then run the server and the client in separate PowerShell terminals:

```powershell
docker compose up -d db
```

```powershell
$env:DATABASE_URL = 'postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable'
go run ./cmd/chat-server
```

```powershell
Set-Location web
npm ci
npm run dev
```

Open the Vite URL, usually `http://localhost:5173`. The dev server proxies `/api` and `/ws` to the Go server on port `8080`.

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
| `GET`  | `/api/rooms?user_id={id}&q={text}`                 | Find rooms by name (all rooms are public)  |
| `GET`  | `/api/rooms/{roomID}/members?user_id={id}`         | List room members; members only            |
| `POST` | `/api/rooms/{roomID}/join`                         | Join a room                                |
| `GET`  | `/api/messages/dm?user_id={id}&with={buddyID}`     | Fetch direct-message history               |
| `GET`  | `/api/messages/room?user_id={id}&room_id={roomID}` | Fetch room history                         |
| `GET`  | `/ws?user_id={id}`                                 | Open a WebSocket connection                |

History endpoints accept an optional `limit` query parameter (default 50, maximum 200) and `before_id` cursor for loading earlier pages. Pages are returned oldest-first. Users must be buddies to exchange or fetch direct messages. Users must be room members to send or fetch room messages.

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

Each connected recipient receives a JSON event with `"type":"message"`. The sender also receives an acknowledgement such as `{ "type": "ack", "message_id": 12, "status": "delivered" }`. `sent` means Postgres saved the message; `delivered` means the server queued it for at least one live recipient. It is not a read receipt. Presence events have `"type":"presence"`; invalid messages and delivery failures return `"type":"error"`.

Send a BUZZ to a buddy without saving a chat message:

```json
{ "type": "buzz", "recipient_id": 2 }
```

The recipient receives `{ "type": "buzz", "user_id": 1 }`. Buddies must be online to receive a buzz.

To show that you are typing, send `{ "type": "typing", "recipient_id": 2 }` or `{ "type": "typing", "room_id": 1 }`. The server does not store it and sends no reply. The buddy, or the other room members, receive `{ "type": "typing", "user_id": 1 }` (with `room_id` for rooms). Resend about every two seconds while typing. Receivers should treat a gap of four seconds as the user having stopped.

### Fetch message history

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/dm?user_id=$($alice.id)&with=$($bob.id)"
Invoke-RestMethod -Uri "http://localhost:8080/api/messages/room?user_id=$($alice.id)&room_id=$($room.id)"
```

## Using the web client

The React, TypeScript, and Tailwind client lives in `web/src/`, organized into `components/`, `api/`, `hooks/`, `types/`, and `utils/`.

Sign in with a username in each window. Choose **Side by side** to run two signed-in users next to each other: a message sent in one window appears in the other as soon as the server delivers it, and the other window shows "is typing..." while the sender types. Use the emoji button to insert a Unicode equivalent of a classic Yahoo emoticon at the cursor. Both users need to be buddies, so add one from the other's window.

Use **Find rooms** in the sidebar to search rooms by name and join one. Every room is public, so any signed-in user can find and join it. Use **Members** in a room's header to see who is in it. Member online status is refreshed every 15 seconds.

## Run checks

The Go checks need Go installed locally; the web checks need Node.

```powershell
go test ./...
go build ./cmd/chat-server
```

```powershell
Set-Location web
npm run lint
npm run build
```

`go test -race ./...` also works, but it needs cgo and a C compiler, which Windows does not have by default.

## Project layout

```text
cmd/chat-server/       Server entry point
web/src/components/    React UI components
web/src/api/           HTTP and WebSocket client functions
web/src/hooks/         Session and connection state
web/src/types/         Shared application and protocol types
web/src/utils/         Pure chat and bot helpers
internal/api/          HTTP and WebSocket endpoints
internal/chat/         Hub, clients, and chat message contracts
internal/bot/          Bot runtime, commands, Gemini responder, and limits
internal/data/         PostgreSQL store and embedded schema
internal/room/         Room domain types and validation
internal/user/         User domain types and validation
docker-compose.yml     Local PostgreSQL, Go server, and web dev server
Dockerfile             Multi-stage Go build
```
