import { useId, useRef, useState, type FormEvent } from "react";
import { api } from "./api";
import type { RoomListing } from "./types";
import type { Session } from "./useSession";
import { inputClass, secondaryButton } from "./Window";

type LoadState = "idle" | "loading" | "ready" | "error";

const RESULT_LIMIT = 50;

export function RoomBrowser({ session }: { session: Session }) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [state, setState] = useState<LoadState>("idle");
  const [results, setResults] = useState<RoomListing[]>([]);
  const [searched, setSearched] = useState("");
  const [joiningId, setJoiningId] = useState<number | null>(null);
  const [joinError, setJoinError] = useState<string | null>(null);
  const latestRequest = useRef(0);

  async function load(text: string) {
    const request = ++latestRequest.current;
    setState("loading");
    try {
      const list = await api.browseRooms(session.user.id, text);
      if (request !== latestRequest.current) return;
      setResults(list);
      setSearched(text);
      setState("ready");
    } catch {
      if (request === latestRequest.current) setState("error");
    }
  }

  function toggle() {
    const next = !open;
    setOpen(next);
    if (next) void load(query.trim());
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void load(query.trim());
  }

  async function join(room: RoomListing) {
    setJoinError(null);
    setJoiningId(room.id);
    try {
      await session.joinRoom(room);
      await load(searched);
    } catch (failure) {
      setJoinError(
        failure instanceof Error ? failure.message : "Could not join the room.",
      );
    } finally {
      setJoiningId(null);
    }
  }

  return (
    <div className="mt-2">
      <button
        type="button"
        className={`${secondaryButton} w-full`}
        aria-expanded={open}
        aria-controls={`${id}-panel`}
        onClick={toggle}
      >
        Find rooms
      </button>

      {open ? (
        <section
          id={`${id}-panel`}
          aria-label="Find rooms"
          className="mt-2 rounded-win border border-line bg-white p-2"
        >
          <form onSubmit={submit} noValidate>
            <label htmlFor={`${id}-search`} className="mb-1 block text-muted">
              Search rooms by name
            </label>
            <div className="flex gap-1">
              <input
                id={`${id}-search`}
                className={inputClass}
                value={query}
                maxLength={100}
                autoComplete="off"
                spellCheck={false}
                onChange={(event) => setQuery(event.target.value)}
              />
              <button
                type="submit"
                className={`${secondaryButton} shrink-0`}
                disabled={state === "loading"}
              >
                Search
              </button>
            </div>
          </form>

          <div aria-live="polite" className="mt-2">
            {state === "loading" ? (
              <p className="text-muted">Searching rooms...</p>
            ) : null}

            {state === "error" ? (
              <div role="alert">
                <p className="font-bold text-danger">Could not search rooms.</p>
                <button
                  type="button"
                  className={`${secondaryButton} mt-2`}
                  onClick={() => void load(query.trim())}
                >
                  Try again
                </button>
              </div>
            ) : null}

            {state === "ready" && results.length === 0 ? (
              <p className="text-muted">
                {searched
                  ? `No rooms match "${searched}".`
                  : "No rooms exist yet. Create the first one below."}
              </p>
            ) : null}

            {state === "ready" && results.length > 0 ? (
              <ul>
                {results.map((room) => (
                  <li
                    key={room.id}
                    className="flex items-center gap-2 border-b border-selected py-1 last:border-b-0"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-bold">
                        {room.name}
                      </span>
                      <span className="text-xs text-muted">
                        {room.member_count}{" "}
                        {room.member_count === 1 ? "member" : "members"}
                      </span>
                    </span>
                    {room.joined ? (
                      <button
                        type="button"
                        className={`${secondaryButton} shrink-0`}
                        aria-label={`Open ${room.name}`}
                        onClick={() => session.select(`room:${room.id}`)}
                      >
                        Open
                      </button>
                    ) : (
                      <button
                        type="button"
                        className={`${secondaryButton} shrink-0`}
                        aria-label={`Join ${room.name}`}
                        disabled={joiningId !== null}
                        onClick={() => void join(room)}
                      >
                        {joiningId === room.id ? "Joining..." : "Join"}
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            ) : null}

            {state === "ready" && results.length >= RESULT_LIMIT ? (
              <p className="mt-1 text-xs text-muted">
                Showing the {RESULT_LIMIT} largest matches. Search to narrow
                them down.
              </p>
            ) : null}
          </div>

          {joinError ? (
            <p role="alert" className="mt-2 font-bold text-danger">
              {joinError}
            </p>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}
