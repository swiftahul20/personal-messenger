export interface User {
  id: number;
  username: string;
  created_at: string;
}

export interface Buddy extends User {
  online: boolean;
}

export interface Room {
  id: number;
  name: string;
  created_at: string;
}

export interface ChatMessage {
  id: number;
  sender_id: number;
  recipient_id?: number;
  room_id?: number;
  content: string;
  sent_at: string;
}

export type ServerEvent =
  | { type: "message"; message: ChatMessage }
  | { type: "presence"; user_id: number; online: boolean }
  | { type: "error"; error: string };
