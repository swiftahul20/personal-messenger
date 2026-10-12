# CyberKid00 — LLM Bot Buddy for the Personal Messenger

A build plan for adding a SmarterChild-style bot to the existing Go + Vue messenger (Yahoo Messenger-inspired). The bot is a buddy named **CyberKid00** that is always online, replies to direct messages in early-2000s chat slang, and is powered by an LLM.

## Context

- The messenger is a monolith: Go backend at the repo root, Vue frontend in `/web`. Deployed on a VPS behind Caddy and Cloudflare at `chat.<domain>`, using a shared Postgres container.
- Backend packages known so far: `internal/api` (HTTP routes, including `/api/healthz`), `internal/chat` (the Hub), `internal/data` (Postgres store, `Migrate` runs at startup).
- Auth is **username-only** (no passwords). Anyone can pick a username and chat. This shapes the abuse-protection requirements below.
- The Hub implementation has not been reviewed for this plan. Read `internal/chat` before wiring anything, because the integration points below assume a hub loop that owns the connected-clients map and routes direct messages and room messages.

## Goals

1. A bot buddy, `CyberKid00`, that is always shown online in every user's buddy list.
2. Users can DM it and get an in-character reply from an LLM.
3. Conversation history works for free through the existing `messages` table.
4. The bot cannot be abused to burn the LLM quota, and it fails gracefully in character.
5. The design teaches concurrency: channels between the hub and the bot, per-user ordering, context cancellation, rate limiting.

## Core Design: The Bot Is Just Another User

- At startup, upsert a `users` row with username `CyberKid00` and keep its ID.
- The hub treats that ID as always online (presence includes it).
- **Reserve the username**: reject registration of `CyberKid00` case-insensitively, so nobody can impersonate it.
- A DM to the bot is persisted like any other message. The bot's replies are ordinary messages with `sender_id` = the bot's ID and `recipient_id` = the user, so history loads like any other DM. No schema change is needed.

## Hub Integration

- When the hub routes a DM and the recipient is the bot, it sends the message to the bot's **inbox channel** instead of looking for a WebSocket client.
- The bot processes messages in its own goroutines and sends its reply back through the hub's existing broadcast channel, so the hub persists and delivers it exactly like a normal message.
- The hub keeps sole ownership of the connected-clients map. The bot never touches it.

**Deadlock rule:** the hub must never block when sending to the bot's inbox. Use a buffered channel and `select { case inbox <- msg: default: }`. If the inbox is full, the bot-side handling replies in character with a "too many msgs, brb" style line. The bot may block when sending to the hub, but the hub never waits on the bot, so the two cannot lock each other.

## Conversation Handling

- Per user, process **one message at a time** (a per-user queue or per-user worker) so replies stay in order.
- Context for each LLM call: the last ~20 messages between that user and the bot, loaded from the `messages` table, plus the persona system prompt.
- Cancel in-flight work through `context.Context` on server shutdown. A reply that finishes after the user disconnects is still stored and appears the next time they open the chat.

## Reply Style

- The bot shows a **"CyberKid00 is typing..."** indicator while the LLM call runs, then delivers the **complete** message.
- This needs one new WebSocket event type, `typing`, sent from server to client. The exact message envelope must be taken from the existing protocol in `internal/chat`.
- Token-by-token streaming is a later, optional phase.

## Persona

Early-2000s instant-messenger voice. Draft rules for the system prompt:

- Replies are 1 to 3 short lines, mostly lowercase.
- Uses slang and abbreviations: u, ur, omg, lol, brb, g2g, ttyl, k thx bye.
- Uses emoticons (`:)` `:D` `:P` `;)` `<3`) and decorative bits like `~*~`.
- Says it is a bot when asked and never pretends to be human.
- **Never asks users for personal information.** If asked "a/s/l?", it answers with a joke such as `1/bot/the internet lol ~*~ ur turn :P` and does not request the user's real details.
- Keeps content friendly and PG.
- If a user seems upset or mentions harming themselves, it drops the slang and responds with care.

Example:

```
you: hey
CyberKid00: heyyy :) whats up?? u bored 2? lol
you: whats the capital of france
CyberKid00: paris!! duh :P do u have the eiffel tower on ur wall too?? jk jk
```

**Away messages** are used as the in-character failure mode (quota hit, LLM error, bot disabled), for example `~*~ brb eating pizza ~*~` or `g2g do hw, ttyl!!`. The bot can show as "away" with one of these instead of returning an error.

## LLM Provider

- Use the **Gemini API free tier** (no credit card), with a Flash-class model. The exact model name should be confirmed when implementing, since model names change.
- Only messages sent to the bot are sent to the LLM, never other DMs or rooms.
- Check Google's current terms: on the free tier, prompts and responses may be used to improve Google products. Show a short note in the UI such as "don't share secrets with CyberKid00".
- Cap output length (max output tokens) so replies stay short and cheap.

## Abuse Protection (required, not optional)

Because auth is username-only and the site is public, anyone can create a user and spam the bot to exhaust the free quota.

- **Per-user rate limit** on messages to the bot (a few per minute).
- **Global daily cap** on LLM calls, tuned to the real free-tier quota (check the current limits; the numbers are not fixed here).
- **`BOT_ENABLED` switch** to turn the bot off without redeploying code.
- When any limit is hit, the bot goes away with an in-character away message instead of calling the LLM.
- Treat all user text as untrusted. In phase 1 the bot has no tools, so prompt injection can only change what it says. Revisit this before adding tools in phase 3.

## Configuration and Secrets

Proposed environment variables, set through the VPS `.env` and mapped in `docker-compose.yml` under `environment:`:

```
GEMINI_API_KEY=...
BOT_ENABLED=true
BOT_USERNAME=CyberKid00
BOT_USER_RPM=...
BOT_DAILY_CAP=...
```

The API key must be provided at **runtime only**. Images are built locally and pushed to Docker Hub, so never bake the key into the image or commit it. The `.dockerignore` already excludes `.env`; keep it that way.

## Code Layout

```
internal/bot/
  bot.go        # inbox channel, per-user queues, worker loop, hub integration
  persona.go    # system prompt and in-character fallback/away lines
  commands.go   # slash commands handled in Go with no LLM
  limiter.go    # per-user rate limit and global daily cap
  llm.go        # Responder interface
  gemini.go     # Gemini implementation of Responder
```

```go
type Responder interface {
    Reply(ctx context.Context, history []Message) (string, error)
}
```

The `Responder` interface lets the whole flow be tested with a fake implementation, so tests never spend quota. Run tests with `go test -race` since this feature is concurrent by design.

## Build Phases

**Phase 1: core bot**
- Bot user upsert, reserved username, always-online presence.
- Hub routes DMs to the bot inbox (non-blocking).
- Gemini replies with the persona prompt and recent history.
- `typing` event, then the full message.
- Per-user limit, daily cap, `BOT_ENABLED`, in-character away fallback.
- Frontend: bot in the buddy list with a "BOT" badge, typing indicator.

**Phase 2: SmarterChild-style commands (no LLM, handled in Go)**
- `/help`, `/8ball`, `/roll [NdM]`, `/time`.
- **BUZZ**: a message type that makes the chat window shake on the client.
- Commands run before the LLM, which saves quota and matches the original bot's command-driven feel.

**Phase 3: agentic features**
- Tool calling for things like a trivia game with state or weather lookups. Add input validation and a max-iterations guard, and re-check the prompt injection risk first.

**Phase 4: optional streaming**
- Stream tokens to the client over the WebSocket instead of a single complete message.

## Open Items

- Read `internal/chat` to confirm hub internals, the WebSocket message envelope, and how presence is broadcast.
- Pick the exact Gemini model and confirm the current free-tier limits.
- Decide the numeric defaults for `BOT_USER_RPM` and `BOT_DAILY_CAP`.
- Decide the frontend look for the buddy-list entry and the BUZZ animation.
