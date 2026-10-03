import { useCallback, useState } from "react";
import { Pane } from "./Pane";
import type { User } from "./types";

type Layout = "single" | "split";

const toggleClass = (active: boolean) =>
  `min-h-11 rounded-win border border-white px-3 text-sm text-white hover:bg-white/15 focus-visible:outline-white sm:min-h-8 ${
    active ? "bg-white/25 font-bold" : ""
  }`;

export default function App() {
  const [layout, setLayout] = useState<Layout>("single");
  const [users, setUsers] = useState<[User | null, User | null]>([null, null]);
  const [activeSlot, setActiveSlot] = useState<0 | 1>(0);
  const [syncVersion, setSyncVersion] = useState(0);

  const onShared = useCallback(() => setSyncVersion((value) => value + 1), []);
  const split = layout === "split";

  function setUser(slot: 0 | 1, user: User | null) {
    setUsers((prev) => (slot === 0 ? [user, prev[1]] : [prev[0], user]));
  }

  const slotClass = [
    split ? (activeSlot === 0 ? "flex" : "hidden md:flex") : "flex",
    split ? (activeSlot === 1 ? "flex" : "hidden md:flex") : "hidden",
  ];

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex flex-wrap items-center gap-x-4 gap-y-2 bg-toolbar px-3 py-2 text-white">
        <h1 className="text-base font-bold">Personal Messenger</h1>
        <div role="group" aria-label="Window layout" className="flex gap-1">
          <button
            type="button"
            className={toggleClass(!split)}
            aria-pressed={!split}
            onClick={() => setLayout("single")}
          >
            Single window
          </button>
          <button
            type="button"
            className={toggleClass(split)}
            aria-pressed={split}
            onClick={() => setLayout("split")}
          >
            Side by side
          </button>
        </div>
        {split ? (
          <div
            role="group"
            aria-label="Visible window"
            className="flex gap-1 md:hidden"
          >
            {([0, 1] as const).map((slot) => (
              <button
                key={slot}
                type="button"
                className={toggleClass(activeSlot === slot)}
                aria-pressed={activeSlot === slot}
                onClick={() => setActiveSlot(slot)}
              >
                Window {slot + 1}
                {users[slot] ? `: ${users[slot].username}` : ""}
              </button>
            ))}
          </div>
        ) : null}
      </header>

      <main
        className={`grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)] gap-3 p-3 ${
          split
            ? "md:grid-cols-[repeat(2,minmax(0,1fr))]"
            : "mx-auto w-full max-w-5xl"
        }`}
      >
        {([0, 1] as const).map((slot) => (
          <div key={slot} className={`min-h-0 min-w-0 ${slotClass[slot]}`}>
            <Pane
              slot={slot + 1}
              user={users[slot]}
              blockedUsername={users[slot === 0 ? 1 : 0]?.username}
              syncVersion={syncVersion}
              onShared={onShared}
              onSignedIn={(user) => setUser(slot, user)}
              onSignOut={() => setUser(slot, null)}
            />
          </div>
        ))}
      </main>
    </div>
  );
}
