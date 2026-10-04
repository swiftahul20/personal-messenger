import { useEffect, useState } from "react";
import { api } from "./api";
import type { RoomMember } from "./types";
import type { Session } from "./useSession";

// Presence is only pushed to buddies, so member status is refreshed by polling.
const REFRESH_MS = 15000;

export function RoomMembers({
  session,
  roomId,
}: {
  session: Session;
  roomId: number;
}) {
  const [members, setMembers] = useState<RoomMember[] | null>(null);
  const [failed, setFailed] = useState(false);
  const { user, syncVersion } = session;

  useEffect(() => {
    let cancelled = false;
    const load = () =>
      api
        .roomMembers(user.id, roomId)
        .then((list) => {
          if (cancelled) return;
          setMembers(list);
          setFailed(false);
        })
        .catch(() => {
          if (!cancelled) setFailed(true);
        });
    void load();
    const timer = window.setInterval(load, REFRESH_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [user.id, roomId, syncVersion]);

  const sorted = members
    ? [...members].sort(
        (a, b) =>
          Number(b.online) - Number(a.online) ||
          a.username.localeCompare(b.username),
      )
    : [];

  return (
    <section
      aria-label="Room members"
      className="max-h-40 overflow-y-auto border-b border-line bg-white px-3 py-2"
    >
      <h4 className="mb-1 font-bold text-brand">
        Members{members ? ` (${members.length})` : ""}
      </h4>

      {members === null && !failed ? (
        <p className="text-muted">Loading members...</p>
      ) : null}

      {failed ? (
        <p role="alert" className="font-bold text-danger">
          Could not refresh members.
        </p>
      ) : null}

      {members ? (
        <ul>
          {sorted.map((member) => (
            <li key={member.id} className="flex min-h-7 items-center gap-2">
              <span
                aria-hidden="true"
                className={`size-2.5 shrink-0 rounded-full border ${
                  member.online
                    ? "border-online bg-online"
                    : "border-line bg-transparent"
                }`}
              />
              <span
                className={`min-w-0 flex-1 truncate ${member.online ? "font-bold" : "text-muted"}`}
              >
                {member.username}
                {member.id === user.id ? " (you)" : ""}
              </span>
              <span className="text-xs text-muted">
                {member.online ? "Online" : "Offline"}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
