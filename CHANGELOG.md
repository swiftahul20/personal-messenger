# Changelog

## Unreleased

### Added

- CyberKid00 can use Gemini tool calling for per-user trivia games and current weather lookups.
- Weather results come from Open-Meteo and include source attribution.
- The server validates tool names and arguments, limits tool calls per reply, and keeps trivia state isolated by user.
- CyberKid00's buddy presence shows Online or Away from keyboard (AFK) based on Gemini availability.

### Changed

- Removed canned fallback DMs. A missing Gemini key or failed Gemini request sets the bot to AFK without sending an automated reply; a successful response restores Online presence.
