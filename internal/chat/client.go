package chat

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8 * 1024
)

type Client struct {
	userID int
	hub    *Hub
	conn   *websocket.Conn
	send   chan Event
}

func NewClient(userID int, hub *Hub, conn *websocket.Conn) *Client {
	return &Client{userID: userID, hub: hub, conn: conn, send: make(chan Event, 32)}
}

func (c *Client) Run() {
	if err := c.hub.Register(c); err != nil {
		_ = c.conn.Close()
		return
	}
	go c.writePump()
	c.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("websocket read for user %d: %v", c.userID, err)
			}
			return
		}
		var message IncomingMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			c.deliver(Event{Type: "error", Error: "invalid JSON message"})
			continue
		}
		if err := c.hub.Publish(message, c.userID); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case event, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(event); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) deliver(event Event) bool {
	select {
	case c.send <- event:
		return true
	default:
		return false
	}
}
