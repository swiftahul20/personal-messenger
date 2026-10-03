import { useCallback, useEffect, useRef, useState } from "react";
import { api, socketUrl } from "./api";
import type { Buddy, ChatMessage, Room, ServerEvent, User } from "./types";

export type Connection = "connecting" | "open" | "closed";
export type LoadState = "loading" | "ready" | "error";
export type Session = ReturnType<typeof useSession>;

function mergeMessages(a: ChatMessage[], b: ChatMessage[]): ChatMessage[] {
  const byId = new Map<number, ChatMessage>();
  for (const message of a) byId.set(message.id, message);
  for (const message of b) byId.set(message.id, message);
  return [...byId.values()].sort((x, y) => x.id - y.id);
}

function parseKey(key: string): { kind: string; id: number } {
  const [kind, rawId] = key.split(":");
  return { kind, id: Number(rawId) };
}

// syncVersion changes when either window adds a buddy or room, so both lists reload.
export function useSession(
  user: User,
  syncVersion: number,
  onShared: () => void,
) {
  const [connection, setConnection] = useState<Connection>("connecting");
  const [attempt, setAttempt] = useState(0);
  const [buddies, setBuddies] = useState<Buddy[]>([]);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [listState, setListState] = useState<LoadState>("loading");
  const [reloadTick, setReloadTick] = useState(0);
  const [messages, setMessages] = useState<Record<string, ChatMessage[]>>({});
  const [history, setHistory] = useState<Record<string, LoadState>>({});
  const [unread, setUnread] = useState<Record<string, number>>({});
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const socketRef = useRef<WebSocket | null>(null);
  const activeRef = useRef<string | null>(null);
  const loadedRef = useRef(new Set<string>());

  useEffect(() => {
    const socket = new WebSocket(socketUrl(user.id));
    socketRef.current = socket;

    socket.onopen = () => setConnection("open");
    socket.onclose = () => setConnection("closed");
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data as string) as ServerEvent;
      if (data.type === "presence") {
        setBuddies((prev) =>
          prev.map((buddy) =>
            buddy.id === data.user_id
              ? { ...buddy, online: data.online }
              : buddy,
          ),
        );
      } else if (data.type === "message") {
        const message = data.message;
        const otherId =
          message.sender_id === user.id
            ? message.recipient_id
            : message.sender_id;
        const key = message.room_id
          ? `room:${message.room_id}`
          : `dm:${otherId}`;
        setMessages((prev) => ({
          ...prev,
          [key]: mergeMessages(prev[key] ?? [], [message]),
        }));
        if (message.sender_id !== user.id && activeRef.current !== key) {
          setUnread((prev) => ({ ...prev, [key]: (prev[key] ?? 0) + 1 }));
        }
      } else if (data.type === "error") {
        setNotice(data.error);
      }
    };

    return () => {
      socket.onclose = null;
      socket.onmessage = null;
      // Closing a socket that is still connecting logs a browser warning.
      if (socket.readyState === WebSocket.CONNECTING) {
        socket.onopen = () => socket.close();
      } else {
        socket.onopen = null;
        socket.close();
      }
    };
  }, [user.id, attempt]);

  // Lists load after the socket opens so the online flags match the server's view.
  useEffect(() => {
    if (connection !== "open") return;
    let cancelled = false;
    Promise.all([api.buddies(user.id), api.rooms(user.id)])
      .then(([nextBuddies, nextRooms]) => {
        if (cancelled) return;
        setBuddies(nextBuddies);
        setRooms(nextRooms);
        setListState("ready");
      })
      .catch(() => {
        if (!cancelled) setListState("error");
      });
    return () => {
      cancelled = true;
    };
  }, [connection, user.id, syncVersion, reloadTick]);

  const loadHistory = useCallback(
    async (key: string) => {
      const { kind, id } = parseKey(key);
      setHistory((prev) => ({ ...prev, [key]: "loading" }));
      try {
        const list =
          kind === "dm"
            ? await api.dmHistory(user.id, id)
            : await api.roomHistory(user.id, id);
        setMessages((prev) => ({
          ...prev,
          [key]: mergeMessages(list, prev[key] ?? []),
        }));
        setHistory((prev) => ({ ...prev, [key]: "ready" }));
      } catch {
        loadedRef.current.delete(key);
        setHistory((prev) => ({ ...prev, [key]: "error" }));
      }
    },
    [user.id],
  );

  const select = useCallback(
    (key: string | null) => {
      activeRef.current = key;
      setActiveKey(key);
      setNotice(null);
      if (!key) return;
      setUnread((prev) => ({ ...prev, [key]: 0 }));
      if (!loadedRef.current.has(key)) {
        loadedRef.current.add(key);
        void loadHistory(key);
      }
    },
    [loadHistory],
  );

  const send = useCallback((key: string, content: string): boolean => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setNotice("Not connected. Reconnect to send messages.");
      return false;
    }
    const { kind, id } = parseKey(key);
    const payload =
      kind === "dm"
        ? { type: "dm", recipient_id: id, content }
        : { type: "room", room_id: id, content };
    socket.send(JSON.stringify(payload));
    setNotice(null);
    return true;
  }, []);

  const addBuddy = useCallback(
    async (username: string) => {
      await api.addBuddy(user.id, username);
      onShared();
    },
    [user.id, onShared],
  );

  const createRoom = useCallback(
    async (name: string) => {
      const room = await api.createRoom(user.id, name);
      setRooms((prev) => [...prev, room]);
      onShared();
      select(`room:${room.id}`);
    },
    [user.id, onShared, select],
  );

  const joinRoom = useCallback(
    async (roomId: number) => {
      await api.joinRoom(user.id, roomId);
      onShared();
    },
    [user.id, onShared],
  );

  const reconnect = useCallback(() => {
    setConnection("connecting");
    setAttempt((value) => value + 1);
  }, []);

  const retryLists = useCallback(() => {
    setListState("loading");
    setReloadTick((value) => value + 1);
  }, []);

  return {
    user,
    connection,
    buddies,
    rooms,
    listState,
    messages,
    history,
    unread,
    activeKey,
    notice,
    select,
    send,
    addBuddy,
    createRoom,
    joinRoom,
    reconnect,
    retryLists,
    retryHistory: loadHistory,
  };
}
