import { useCallback, useEffect, useRef, useState } from "react";
import { api, HISTORY_PAGE_SIZE, socketUrl } from "./api";
import type {
  Buddy,
  ChatMessage,
  DeliveryStatus,
  Room,
  ServerEvent,
  User,
} from "./types";

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

// The sender repeats typing events while the draft changes, so a gap longer than this means they stopped.
const TYPING_EXPIRES_MS = 4000;
const TYPING_SEND_EVERY_MS = 2000;
const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;

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
  const [hasOlder, setHasOlder] = useState<Record<string, boolean>>({});
  const [olderLoading, setOlderLoading] = useState<Record<string, boolean>>({});
  const [delivery, setDelivery] = useState<Record<number, DeliveryStatus>>({});
  const [unread, setUnread] = useState<Record<string, number>>({});
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [typing, setTyping] = useState<Record<string, number[]>>({});

  const socketRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<number | null>(null);
  const activeRef = useRef<string | null>(null);
  const loadedRef = useRef(new Set<string>());
  const typingTimers = useRef(new Map<string, number>());
  const lastTypingSent = useRef(new Map<string, number>());

  const clearTyping = useCallback((key: string, userId: number) => {
    const timerKey = `${key}:${userId}`;
    window.clearTimeout(typingTimers.current.get(timerKey));
    typingTimers.current.delete(timerKey);
    setTyping((prev) => {
      if (!prev[key]?.includes(userId)) return prev;
      return { ...prev, [key]: prev[key].filter((id) => id !== userId) };
    });
  }, []);

  const markTyping = useCallback(
    (key: string, userId: number) => {
      const timerKey = `${key}:${userId}`;
      window.clearTimeout(typingTimers.current.get(timerKey));
      typingTimers.current.set(
        timerKey,
        window.setTimeout(() => clearTyping(key, userId), TYPING_EXPIRES_MS),
      );
      setTyping((prev) =>
        prev[key]?.includes(userId)
          ? prev
          : { ...prev, [key]: [...(prev[key] ?? []), userId] },
      );
    },
    [clearTyping],
  );

  useEffect(() => {
    const timers = typingTimers.current;
    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
      timers.clear();
    };
  }, [user.id, attempt]);

  useEffect(() => {
    let disposed = false;
    const socket = new WebSocket(socketUrl(user.id));
    socketRef.current = socket;

    socket.onopen = () => {
      if (disposed) return;
      reconnectAttemptRef.current = 0;
      if (reconnectTimerRef.current !== null) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      setConnection("open");
    };
    socket.onclose = () => {
      if (disposed) return;
      setConnection("closed");
      const delay = Math.min(
        RECONNECT_BASE_MS * 2 ** reconnectAttemptRef.current,
        RECONNECT_MAX_MS,
      );
      reconnectAttemptRef.current += 1;
      reconnectTimerRef.current = window.setTimeout(() => {
        reconnectTimerRef.current = null;
        if (!disposed) {
          setConnection("connecting");
          setAttempt((value) => value + 1);
        }
      }, delay);
    };
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data as string) as ServerEvent;
      if (data.type === "ack") {
        setDelivery((prev) => ({ ...prev, [data.message_id]: data.status }));
      } else if (data.type === "presence") {
        setBuddies((prev) =>
          prev.map((buddy) =>
            buddy.id === data.user_id
              ? { ...buddy, online: data.online }
              : buddy,
          ),
        );
      } else if (data.type === "typing") {
        const key = data.room_id
          ? `room:${data.room_id}`
          : `dm:${data.user_id}`;
        markTyping(key, data.user_id);
      } else if (data.type === "message") {
        const message = data.message;
        const otherId =
          message.sender_id === user.id
            ? message.recipient_id
            : message.sender_id;
        const key = message.room_id
          ? `room:${message.room_id}`
          : `dm:${otherId}`;
        clearTyping(key, message.sender_id);
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
      disposed = true;
      if (reconnectTimerRef.current !== null) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
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
  }, [user.id, attempt, markTyping, clearTyping]);

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
        setHasOlder((prev) => ({
          ...prev,
          [key]: list.length > HISTORY_PAGE_SIZE,
        }));
        setMessages((prev) => ({
          ...prev,
          [key]: mergeMessages(list.slice(-HISTORY_PAGE_SIZE), prev[key] ?? []),
        }));
        setHistory((prev) => ({ ...prev, [key]: "ready" }));
      } catch {
        loadedRef.current.delete(key);
        setHistory((prev) => ({ ...prev, [key]: "error" }));
      }
    },
    [user.id],
  );

  const loadOlder = useCallback(
    async (key: string): Promise<boolean> => {
      const oldestId = messages[key]?.[0]?.id;
      if (!oldestId || !hasOlder[key] || olderLoading[key]) return false;

      const { kind, id } = parseKey(key);
      setOlderLoading((prev) => ({ ...prev, [key]: true }));
      try {
        const older =
          kind === "dm"
            ? await api.dmHistory(user.id, id, oldestId)
            : await api.roomHistory(user.id, id, oldestId);
        const page = older.slice(-HISTORY_PAGE_SIZE);
        setMessages((prev) => ({
          ...prev,
          [key]: mergeMessages(page, prev[key] ?? []),
        }));
        setHasOlder((prev) => ({
          ...prev,
          [key]: older.length > HISTORY_PAGE_SIZE,
        }));
        return page.length > 0;
      } catch {
        setNotice("Could not load older messages.");
        return false;
      } finally {
        setOlderLoading((prev) => ({ ...prev, [key]: false }));
      }
    },
    [hasOlder, messages, olderLoading, user.id],
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

  const sendTyping = useCallback((key: string) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    const now = Date.now();
    if (now - (lastTypingSent.current.get(key) ?? 0) < TYPING_SEND_EVERY_MS) {
      return;
    }
    lastTypingSent.current.set(key, now);
    const { kind, id } = parseKey(key);
    socket.send(
      JSON.stringify(
        kind === "dm"
          ? { type: "typing", recipient_id: id }
          : { type: "typing", room_id: id },
      ),
    );
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
    async (room: Room) => {
      await api.joinRoom(user.id, room.id);
      setRooms((prev) =>
        prev.some((item) => item.id === room.id) ? prev : [...prev, room],
      );
      onShared();
      select(`room:${room.id}`);
    },
    [user.id, onShared, select],
  );

  const reconnect = useCallback(() => {
    reconnectAttemptRef.current = 0;
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
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
    hasOlder,
    olderLoading,
    delivery,
    unread,
    activeKey,
    notice,
    typing,
    select,
    send,
    sendTyping,
    addBuddy,
    createRoom,
    joinRoom,
    syncVersion,
    reconnect,
    retryLists,
    retryHistory: loadHistory,
    loadOlder,
  };
}
