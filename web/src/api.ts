import type {
  Buddy,
  ChatMessage,
  Room,
  RoomListing,
  RoomMember,
  User,
} from "./types";

export const HISTORY_PAGE_SIZE = 100;
const HISTORY_FETCH_SIZE = HISTORY_PAGE_SIZE + 1;

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      ...init,
      headers: { "Content-Type": "application/json" },
    });
  } catch {
    throw new Error(
      "Cannot reach the server. Check that it is running on port 8080.",
    );
  }

  const body: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    if (body === null && response.status >= 500) {
      throw new Error(
        "Cannot reach the server. Check that it is running on port 8080.",
      );
    }
    const message =
      typeof body === "object" && body !== null && "error" in body
        ? String((body as { error: unknown }).error)
        : `Request failed (${response.status})`;
    throw new Error(message);
  }
  return body as T;
}

const post = <T>(path: string, data: unknown) =>
  request<T>(path, { method: "POST", body: JSON.stringify(data) });

export const api = {
  signIn: (username: string) => post<User>("/api/users", { username }),
  buddies: (userId: number) => request<Buddy[]>(`/api/users/${userId}/buddies`),
  addBuddy: (userId: number, username: string) =>
    post<User>(`/api/users/${userId}/buddies`, { username }),
  rooms: (userId: number) => request<Room[]>(`/api/users/${userId}/rooms`),
  createRoom: (userId: number, name: string) =>
    post<Room>("/api/rooms", { user_id: userId, name }),
  browseRooms: (userId: number, query: string) =>
    request<RoomListing[]>(
      `/api/rooms?user_id=${userId}&q=${encodeURIComponent(query)}`,
    ),
  roomMembers: (userId: number, roomId: number) =>
    request<RoomMember[]>(`/api/rooms/${roomId}/members?user_id=${userId}`),
  joinRoom: (userId: number, roomId: number) =>
    post<{ status: string }>(`/api/rooms/${roomId}/join`, { user_id: userId }),
  dmHistory: (userId: number, withId: number, beforeId?: number) =>
    request<ChatMessage[]>(
      `/api/messages/dm?user_id=${userId}&with=${withId}&limit=${HISTORY_FETCH_SIZE}${beforeId ? `&before_id=${beforeId}` : ""}`,
    ),
  roomHistory: (userId: number, roomId: number, beforeId?: number) =>
    request<ChatMessage[]>(
      `/api/messages/room?user_id=${userId}&room_id=${roomId}&limit=${HISTORY_FETCH_SIZE}${beforeId ? `&before_id=${beforeId}` : ""}`,
    ),
};

export function socketUrl(userId: number): string {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return `${scheme}://${window.location.host}/ws?user_id=${userId}`;
}
