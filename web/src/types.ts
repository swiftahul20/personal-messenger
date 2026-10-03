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
  sender_name?: string;
  recipient_id?: number;
  room_id?: number;
  content: string;
  sent_at: string;
}

export type DeliveryStatus = "sent" | "delivered";

export type ServerEvent =
  | { type: "message"; message: ChatMessage }
  | { type: "ack"; message_id: number; status: DeliveryStatus }
  | { type: "presence"; user_id: number; online: boolean }
  | { type: "typing"; user_id: number; room_id?: number }
  | { type: "error"; error: string };
