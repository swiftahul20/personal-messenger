import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import type { ChatMessage } from "./types";
import type { Session } from "./useSession";
import { primaryButton, secondaryButton } from "./Window";

const GROUP_WINDOW_MS = 5 * 60 * 1000;
const EMOTICONS = [
  { emoji: "🙂", code: ":)", label: "Smile" },
  { emoji: "😀", code: ":D", label: "Big grin" },
  { emoji: "😉", code: ";)", label: "Wink" },
  { emoji: "🙁", code: ":(", label: "Sad" },
  { emoji: "😢", code: ":((", label: "Crying" },
  { emoji: "😛", code: ":P", label: "Tongue out" },
  { emoji: "😮", code: ":-O", label: "Surprised" },
  { emoji: "😎", code: "B-)", label: "Cool" },
];

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function Transcript({
  session,
  list,
}: {
  session: Session;
  list: ChatMessage[];
}) {
  const { user, buddies } = session;
  const names = new Map(buddies.map((buddy) => [buddy.id, buddy.username]));
  const nameOf = (id: number) =>
    id === user.id ? user.username : (names.get(id) ?? `User ${id}`);

  return (
    <>
      {list.map((message, index) => {
        const previous = list[index - 1];
        const grouped =
          previous !== undefined &&
          previous.sender_id === message.sender_id &&
          new Date(message.sent_at).getTime() -
            new Date(previous.sent_at).getTime() <
            GROUP_WINDOW_MS;
        const mine = message.sender_id === user.id;
        return (
          <div
            key={message.id}
            className={grouped ? "mt-0.5" : "mt-3 first:mt-0"}
          >
            {grouped ? null : (
              <p className="flex items-baseline gap-2">
                <span
                  className={`font-bold ${mine ? "text-brand" : "text-ink"}`}
                >
                  {nameOf(message.sender_id)}
                </span>
                <time dateTime={message.sent_at} className="text-xs text-muted">
                  {formatTime(message.sent_at)}
                </time>
              </p>
            )}
            <p className="whitespace-pre-wrap wrap-break-word">
              {message.content}
            </p>
          </div>
        );
      })}
    </>
  );
}

export function Conversation({
  session,
  slot,
  className,
  onBack,
}: {
  session: Session;
  slot: number;
  className: string;
  onBack: () => void;
}) {
  const { activeKey, buddies, rooms, messages, history, connection, notice } =
    session;
  const [draft, setDraft] = useState("");
  const [emoticonsOpen, setEmoticonsOpen] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);

  const list = activeKey ? (messages[activeKey] ?? []) : [];
  useEffect(() => {
    const log = logRef.current;
    if (log) log.scrollTop = log.scrollHeight;
  }, [list.length, activeKey]);

  if (!activeKey) {
    return (
      <div
        className={`min-h-0 flex-1 items-center justify-center p-4 text-center text-muted ${className}`}
      >
        <p>Choose a buddy or a room to start chatting.</p>
      </div>
    );
  }

  const [kind, rawId] = activeKey.split(":");
  const id = Number(rawId);
  const buddy =
    kind === "dm" ? buddies.find((item) => item.id === id) : undefined;
  const room =
    kind === "room" ? rooms.find((item) => item.id === id) : undefined;
  const title = buddy?.username ?? room?.name ?? "Conversation";
  const subtitle = buddy
    ? buddy.online
      ? "Online"
      : "Offline"
    : room
      ? `Room ID ${room.id}`
      : "";
  const state = history[activeKey] ?? "loading";
  const connected = connection === "open";
  const composerId = `message-${slot}`;
  const emoticonListId = `emoticons-${slot}`;

  const typingNames = (session.typing[activeKey] ?? []).map(
    (userId) =>
      buddies.find((item) => item.id === userId)?.username ?? `User ${userId}`,
  );
  const typingLabel =
    typingNames.length === 0
      ? ""
      : typingNames.length === 1
        ? `${typingNames[0]} is typing...`
        : typingNames.length === 2
          ? `${typingNames[0]} and ${typingNames[1]} are typing...`
          : "Several people are typing...";

  function submit(event?: FormEvent) {
    event?.preventDefault();
    const content = draft.trim();
    if (!content || !activeKey) return;
    if (session.send(activeKey, content)) setDraft("");
  }

  function onKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      submit();
    }
  }

  function insertEmoticon(text: string) {
    const composer = composerRef.current;
    const start = composer?.selectionStart ?? draft.length;
    const end = composer?.selectionEnd ?? start;
    const nextDraft = `${draft.slice(0, start)}${text}${draft.slice(end)}`;
    if (nextDraft.length > 4000) return;

    setDraft(nextDraft);
    if (nextDraft.trim() && activeKey) session.sendTyping(activeKey);
    setEmoticonsOpen(false);
    requestAnimationFrame(() => {
      composer?.focus();
      composer?.setSelectionRange(start + text.length, start + text.length);
    });
  }

  return (
    <div className={`min-h-0 min-w-0 flex-1 flex-col ${className}`}>
      <div className="flex items-center gap-2 border-b border-line bg-panel px-3 py-2">
        <button
          type="button"
          className={`${secondaryButton} @xl:hidden`}
          onClick={onBack}
        >
          Back
        </button>
        <h3 className="min-w-0 flex-1 truncate font-bold text-brand">
          {title}
        </h3>
        <span className="text-xs text-muted">{subtitle}</span>
      </div>

      <div
        ref={logRef}
        role="log"
        aria-label={`Messages with ${title}`}
        className="min-h-0 flex-1 overflow-y-auto p-3"
      >
        {state === "loading" ? (
          <p className="text-muted">Loading messages...</p>
        ) : null}
        {state === "error" ? (
          <div role="alert">
            <p className="font-bold text-danger">
              Could not load message history.
            </p>
            <button
              type="button"
              className={`${secondaryButton} mt-2`}
              onClick={() => void session.retryHistory(activeKey)}
            >
              Try again
            </button>
          </div>
        ) : null}
        {state === "ready" && list.length === 0 ? (
          <p className="text-muted">
            No messages yet.{" "}
            {buddy
              ? `Say hello to ${buddy.username}.`
              : "Write the first message in this room."}
          </p>
        ) : null}
        {list.length > 0 ? <Transcript session={session} list={list} /> : null}
      </div>

      <p role="status" className="min-h-5 px-3 text-xs text-muted">
        {typingLabel}
      </p>

      <form onSubmit={submit} className="border-t border-line bg-panel p-2">
        {notice ? (
          <p role="alert" className="mb-2 font-bold text-danger">
            {notice}
          </p>
        ) : null}
        {!connected ? (
          <p className="mb-2 text-muted">
            {connection === "connecting" ? "Connecting..." : "Disconnected."}{" "}
            {connection === "closed" ? (
              <button
                type="button"
                className="text-brand underline"
                onClick={session.reconnect}
              >
                Reconnect
              </button>
            ) : null}
          </p>
        ) : null}
        <label htmlFor={composerId} className="sr-only">
          Message to {title}
        </label>
        <div className="flex items-end gap-2">
          <textarea
            ref={composerRef}
            id={composerId}
            rows={2}
            maxLength={4000}
            value={draft}
            onChange={(event) => {
              setDraft(event.target.value);
              if (event.target.value.trim()) session.sendTyping(activeKey);
            }}
            onKeyDown={onKeyDown}
            disabled={!connected}
            className="min-h-11 min-w-0 flex-1 resize-none rounded-win border border-line bg-white p-2 text-ink disabled:bg-selected"
          />
          <div className="relative shrink-0">
            <button
              type="button"
              className={secondaryButton}
              aria-label="Choose an emoji"
              aria-expanded={emoticonsOpen}
              aria-controls={emoticonListId}
              title="Choose a Unicode emoji"
              disabled={!connected}
              onClick={() => setEmoticonsOpen((open) => !open)}
            >
              🙂
            </button>
            {emoticonsOpen ? (
              <div
                id={emoticonListId}
                role="group"
                aria-label="Unicode emoji from classic Yahoo emoticons"
                className="absolute right-0 bottom-full z-10 mb-1 grid w-52 grid-cols-4 gap-1 rounded-win border border-line bg-white p-1 shadow-window"
              >
                {EMOTICONS.map((emoticon) => (
                  <button
                    key={emoticon.code}
                    type="button"
                    className={`${secondaryButton} min-h-11! min-w-11! px-2 font-bold`}
                    aria-label={`${emoticon.label}, ${emoticon.code}`}
                    title={`${emoticon.label} (${emoticon.code})`}
                    disabled={draft.length + emoticon.emoji.length > 4000}
                    onClick={() => insertEmoticon(emoticon.emoji)}
                  >
                    {emoticon.emoji}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          <button
            type="submit"
            className={primaryButton}
            disabled={!connected || !draft.trim()}
          >
            Send
          </button>
        </div>
        <p className="mt-1 text-xs text-muted">
          Enter sends. Shift and Enter adds a new line.
        </p>
      </form>
    </div>
  );
}
