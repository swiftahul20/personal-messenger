import { useEffect, useRef, useState } from "react";
import { BuddyList } from "./BuddyList";
import { Conversation } from "./Conversation";
import { SignIn } from "./SignIn";
import type { User } from "../types";
import { useSession, type Connection } from "../hooks/useSession";
import { titleButton, Window } from "./Window";

const CONNECTION_TEXT: Record<Connection, string> = {
  connecting: "Connecting...",
  open: "Connected",
  closed: "Reconnecting...",
};

interface MessengerProps {
  slot: number;
  user: User;
  syncVersion: number;
  onShared: () => void;
  onSignOut: () => void;
}

function Messenger({
  slot,
  user,
  syncVersion,
  onShared,
  onSignOut,
}: MessengerProps) {
  const session = useSession(user, syncVersion, onShared);
  const windowRef = useRef<HTMLElement | null>(null);
  // Narrow windows show the list or the conversation. backedFrom remembers which conversation was left.
  const [backedFrom, setBackedFrom] = useState<string | null>(null);
  const showList =
    session.activeKey === null || backedFrom === session.activeKey;
  const buzzSender = session.buzz.userId
    ? session.buddies.find((buddy) => buddy.id === session.buzz.userId)
        ?.username ?? `User ${session.buzz.userId}`
    : "";

  useEffect(() => {
    const element = windowRef.current;
    if (
      session.buzz.version === 0 ||
      !element ||
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      return;
    }
    const animation = element.animate(
      [
        { transform: "translateX(0)" },
        { transform: "translateX(-5px)" },
        { transform: "translateX(5px)" },
        { transform: "translateX(-5px)" },
        { transform: "translateX(5px)" },
        { transform: "translateX(0)" },
      ],
      { duration: 650, easing: "ease-in-out" },
    );
    return () => animation.cancel();
  }, [session.buzz.version]);

  const paneSession = {
    ...session,
    select: (key: string | null) => {
      setBackedFrom(null);
      session.select(key);
    },
  };

  return (
    <Window
      title={user.username}
      status={CONNECTION_TEXT[session.connection]}
      label={`Window ${slot}, signed in as ${user.username}`}
      className="h-full w-full"
      elementRef={windowRef}
      actions={
        <button type="button" className={titleButton} onClick={onSignOut}>
          Sign out
        </button>
      }
    >
      {buzzSender ? (
        <p role="status" className="sr-only">
          Buzz from {buzzSender}.
        </p>
      ) : null}
      <div className="@container flex min-h-0 flex-1">
        <BuddyList
          session={paneSession}
          className={showList ? "flex @xl:flex" : "hidden @xl:flex"}
        />
        <Conversation
          session={paneSession}
          slot={slot}
          className={showList ? "hidden @xl:flex" : "flex @xl:flex"}
          onBack={() => setBackedFrom(session.activeKey)}
        />
      </div>
    </Window>
  );
}

interface PaneProps extends Omit<MessengerProps, "user"> {
  user: User | null;
  blockedUsername?: string;
  onSignedIn: (user: User) => void;
}

export function Pane({
  user,
  blockedUsername,
  onSignedIn,
  ...rest
}: PaneProps) {
  if (!user) {
    return (
      <SignIn
        slot={rest.slot}
        blockedUsername={blockedUsername}
        onSignedIn={onSignedIn}
      />
    );
  }
  return <Messenger user={user} {...rest} />;
}
