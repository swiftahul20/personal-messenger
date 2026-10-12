import type { ChatMessage } from "../types";

export function mergeMessages(
  existing: ChatMessage[],
  incoming: ChatMessage[],
): ChatMessage[] {
  const byId = new Map<number, ChatMessage>();
  for (const message of existing) byId.set(message.id, message);
  for (const message of incoming) byId.set(message.id, message);
  return [...byId.values()].sort((a, b) => a.id - b.id);
}

export function parseConversationKey(
  key: string,
): { kind: "dm" | "room"; id: number } {
  const [kind, rawId] = key.split(":");
  const id = Number(rawId);
  if (
    (kind !== "dm" && kind !== "room") ||
    !Number.isSafeInteger(id) ||
    id <= 0
  ) {
    throw new Error(`Invalid conversation key: ${key}`);
  }
  return { kind, id };
}

export function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
}
