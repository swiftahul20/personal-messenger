export interface User {
  id: number;
  username: string;
  created_at: string;
}

export interface Buddy extends User {
  online: boolean;
  presence?: string;
}

export interface Room {
  id: number;
  name: string;
  created_at: string;
}

export interface RoomListing extends Room {
  member_count: number;
  joined: boolean;
}

export interface RoomMember {
  id: number;
  username: string;
  online: boolean;
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
  | { type: "presence"; user_id: number; online: boolean; presence?: string }
  | { type: "typing"; user_id: number; room_id?: number }
  | { type: "buzz"; user_id: number }
  | {
      type: "bot_stream";
      user_id: number;
      recipient_id: number;
      stream_id: string;
      content?: string;
      done?: boolean;
    }
  | { type: "error"; error: string };
