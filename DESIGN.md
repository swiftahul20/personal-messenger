# Design direction

Direction for the React client in `web/`. The brief below is the owner's. The palette, type, and component choices are derived from it and are open to correction.

## Brief (from the owner)

- Style and color follow Yahoo Messenger.
- A user signs in, then chats like in a usual messenger app.
- The user can choose a side-by-side mode: two chat windows next to each other. When the left user sends a message, it shows immediately in the right user's window.
- Logo: plain text for now, no mark.
- Typing indicator: show while the other person is typing.
- Chat composer: include a compact picker for Unicode emoji mapped from classic Yahoo emoticon codes. Use the mappings in the owner's reference issue at `https://github.com/Crissov/unicode-proposals/issues/255`; send Unicode text, not GIF artwork or rich message content, so they work with the existing message storage.

## Design Read

Reading this as: a desktop-first messenger app for people testing a chat server, in a classic 2000s messenger window style (purple title bars, white content, one yellow action color), dial ENERGY 2 / RHYTHM 1 / MOTION 1.

- ENERGY 2: strong purple window chrome, calm content areas.
- RHYTHM 1: both windows are the same app on purpose. Uniformity is what makes the left/right comparison readable.
- MOTION 1: hover and focus states only. No entrance animation, no loops.

## Palette

Core colors: purple and lavender. Accent: yellow. Neutrals: white and ink.

| Role          | Value                           | Reason                                                                                                                                       |
| ------------- | ------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Toolbar       | `#2E0A54`                       | Darkest purple, frames the two windows. White text is 16.31:1.                                                                               |
| Title bar     | gradient `#7A33C4` to `#4F1590` | Classic messenger window chrome. The only gradient, used only to separate window chrome from content. White text is 6.78:1 at the light end. |
| Window purple | `#5B1A9A`                       | Own name in transcripts, selected states, unread count. 10.20:1 on white.                                                                    |
| Panel         | `#F1EBF8`                       | Buddy list and header surfaces.                                                                                                              |
| Selected row  | `#E2D6F0`                       | Current conversation. Ink is 12.76:1 on it.                                                                                                  |
| Desk          | `#CDBDE4`                       | Page background behind the windows.                                                                                                          |
| Ink           | `#1E1230`                       | Body text.                                                                                                                                   |
| Muted         | `#554A68`                       | Secondary text. 7.01:1 on panel, 5.89:1 on selected row.                                                                                     |
| Line          | `#7C6D96`                       | Input borders and the offline dot. 4.68:1 on white, 4.01:1 on panel.                                                                         |
| Accent yellow | `#FFD02B`                       | Primary action buttons only (Sign in, Send). Ink on it is 12.08:1.                                                                           |
| Error         | `#A3122A`                       | Error text only, always with words. 7.84:1 on white.                                                                                         |

Light theme only. The identity is a bright messenger window, so there is no dark mode to ship or break.

## Type

Tahoma, then Verdana, then Segoe UI. Tahoma and Verdana are the faces messenger clients of this era used on Windows, and they need no download.

## Shape and depth

- Radius: 3px on windows, inputs, and buttons. No pills.
- Shadow: one, on the two windows, to lift them off the desk. Nothing else has a shadow.
- No glow, no glass, no background pattern.

## Layout

- Toolbar: plain-text wordmark, a Single / Side by side choice, and on narrow screens a switch between the two windows.
- Each window is a full messenger: title bar with the signed-in name, connection state, and Sign out; buddy and room list; conversation.
- A window shows the list and the conversation together when it is wide enough, and one at a time with a Back button when it is not.
- Below the `md` breakpoint, side by side shows one window at a time. Both stay connected.

## Presence and status

Online is a filled purple dot plus the word "Online". Offline is a hollow dot plus the word "Offline". The dot is never the only signal.

## Chat transcript

Lines, not bubbles. Name in bold, time in muted text, message below. Consecutive messages from one sender within five minutes share one name line.

## Out of scope for now

Away messages, rich content, a logo, avatars, and any real authentication.
