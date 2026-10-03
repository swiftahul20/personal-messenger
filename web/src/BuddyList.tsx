import { useId, useState, type FormEvent } from "react";
import type { Session } from "./useSession";
import { inputClass, secondaryButton } from "./Window";

interface AddFormProps {
  label: string;
  button: string;
  pendingButton: string;
  inputMode?: "text" | "numeric";
  onSubmit: (value: string) => Promise<void>;
}

function AddForm({
  label,
  button,
  pendingButton,
  inputMode = "text",
  onSubmit,
}: AddFormProps) {
  const id = useId();
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = value.trim();
    if (!trimmed) {
      setError(`Enter ${label.toLowerCase()} first.`);
      return;
    }
    setError(null);
    setPending(true);
    try {
      await onSubmit(trimmed);
      setValue("");
    } catch (failure) {
      setError(
        failure instanceof Error ? failure.message : "Something went wrong.",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={submit} className="mt-2" noValidate>
      <label htmlFor={id} className="mb-1 block text-muted">
        {label}
      </label>
      <div className="flex gap-1">
        <input
          id={id}
          className={inputClass}
          value={value}
          inputMode={inputMode}
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          onChange={(event) => setValue(event.target.value)}
          aria-describedby={error ? `${id}-error` : undefined}
          aria-invalid={error ? true : undefined}
        />
        <button
          type="submit"
          className={`${secondaryButton} shrink-0`}
          disabled={pending}
        >
          {pending ? pendingButton : button}
        </button>
      </div>
      {error ? (
        <p
          id={`${id}-error`}
          role="alert"
          className="mt-1 font-bold text-danger"
        >
          {error}
        </p>
      ) : null}
    </form>
  );
}

function Unread({ count }: { count: number }) {
  if (!count) return null;
  return (
    <span className="min-w-5 rounded-win bg-brand px-1.5 text-center text-xs font-bold text-white">
      {count}
      <span className="sr-only"> unread</span>
    </span>
  );
}

const rowClass = (selected: boolean) =>
  `flex min-h-11 w-full items-center gap-2 rounded-win px-2 text-left hover:bg-selected sm:min-h-9 ${
    selected ? "bg-selected" : ""
  }`;

export function BuddyList({
  session,
  className,
}: {
  session: Session;
  className: string;
}) {
  const { buddies, rooms, listState, unread, activeKey, select } = session;

  const sorted = [...buddies].sort(
    (a, b) =>
      Number(b.online) - Number(a.online) ||
      a.username.localeCompare(b.username),
  );

  return (
    <aside
      aria-label="Buddies and rooms"
      className={`min-h-0 flex-col overflow-y-auto border-line bg-panel p-2 @xl:w-56 @xl:shrink-0 @xl:border-r ${className}`}
    >
      {listState === "loading" ? (
        <p className="p-2 text-muted">Loading buddies and rooms...</p>
      ) : null}

      {listState === "error" ? (
        <div className="p-2" role="alert">
          <p className="font-bold text-danger">
            Could not load buddies and rooms.
          </p>
          <button
            type="button"
            className={`${secondaryButton} mt-2`}
            onClick={session.retryLists}
          >
            Try again
          </button>
        </div>
      ) : null}

      {listState === "ready" ? (
        <>
          <h3 className="px-2 pt-1 pb-1 font-bold text-brand">
            Buddies ({buddies.length})
          </h3>
          {sorted.length === 0 ? (
            <p className="px-2 pb-1 text-muted">
              No buddies yet. Add someone by username. They must have signed in
              once.
            </p>
          ) : (
            <ul>
              {sorted.map((buddy) => {
                const key = `dm:${buddy.id}`;
                return (
                  <li key={buddy.id}>
                    <button
                      type="button"
                      className={rowClass(activeKey === key)}
                      aria-current={activeKey === key ? "true" : undefined}
                      onClick={() => select(key)}
                    >
                      <span
                        aria-hidden="true"
                        className={`size-2.5 shrink-0 rounded-full border border-brand ${
                          buddy.online
                            ? "bg-brand"
                            : "border-line bg-transparent"
                        }`}
                      />
                      <span
                        className={`min-w-0 flex-1 truncate ${buddy.online ? "font-bold" : "text-muted"}`}
                      >
                        {buddy.username}
                      </span>
                      <span className="text-xs text-muted">
                        {buddy.online ? "Online" : "Offline"}
                      </span>
                      <Unread count={unread[key] ?? 0} />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
          <AddForm
            label="Add buddy by username"
            button="Add buddy"
            pendingButton="Adding..."
            onSubmit={session.addBuddy}
          />

          <h3 className="mt-4 px-2 pb-1 font-bold text-brand">
            Rooms ({rooms.length})
          </h3>
          {rooms.length === 0 ? (
            <p className="px-2 pb-1 text-muted">
              No rooms yet. Create one, or join with a room ID.
            </p>
          ) : (
            <ul>
              {rooms.map((room) => {
                const key = `room:${room.id}`;
                return (
                  <li key={room.id}>
                    <button
                      type="button"
                      className={rowClass(activeKey === key)}
                      aria-current={activeKey === key ? "true" : undefined}
                      onClick={() => select(key)}
                    >
                      <span className="min-w-0 flex-1 truncate font-bold">
                        {room.name}
                      </span>
                      <span className="text-xs text-muted">ID {room.id}</span>
                      <Unread count={unread[key] ?? 0} />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
          <details className="mt-2">
            <summary className="min-h-11 cursor-pointer px-2 py-2 text-brand sm:min-h-9">
              Create or join a room
            </summary>
            <AddForm
              label="New room name"
              button="Create room"
              pendingButton="Creating..."
              onSubmit={session.createRoom}
            />
            <AddForm
              label="Join by room ID"
              button="Join room"
              pendingButton="Joining..."
              inputMode="numeric"
              onSubmit={async (value) => {
                const id = Number(value);
                if (!Number.isInteger(id) || id <= 0)
                  throw new Error("A room ID is a whole number.");
                await session.joinRoom(id);
              }}
            />
          </details>
        </>
      ) : null}
    </aside>
  );
}
