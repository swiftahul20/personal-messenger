package chat

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct {
	buddies map[[2]int]bool
	rooms   map[int][]int
	users   map[int]string
}

func (f fakeStore) SaveMessage(_ context.Context, message *Message) error {
	message.SenderName = f.users[message.SenderID]
	return nil
}

func (f fakeStore) AreBuddies(_ context.Context, a, b int) (bool, error) {
	return f.buddies[[2]int{a, b}] || f.buddies[[2]int{b, a}], nil
}

func (f fakeStore) IsRoomMember(_ context.Context, roomID, userID int) (bool, error) {
	for _, id := range f.rooms[roomID] {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}

func (f fakeStore) RoomMemberIDs(_ context.Context, roomID int) ([]int, error) {
	return f.rooms[roomID], nil
}

func (f fakeStore) BuddyIDs(context.Context, int) ([]int, error) { return nil, nil }

func (f fakeStore) DMHistory(context.Context, int, int, int) ([]Message, error) { return nil, nil }

func (f fakeStore) RoomHistory(context.Context, int, int) ([]Message, error) { return nil, nil }

func startHub(t *testing.T, store Store, userIDs ...int) (*Hub, map[int]*Client) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub := NewHub(store)
	go hub.Run(ctx)

	clients := make(map[int]*Client, len(userIDs))
	for _, id := range userIDs {
		client := &Client{userID: id, hub: hub, send: make(chan Event, 8)}
		if err := hub.Register(client); err != nil {
			t.Fatal(err)
		}
		clients[id] = client
	}
	hub.OnlineUsers() // the hub handles requests in order, so this waits for the registrations
	return hub, clients
}

func receive(c *Client) (Event, bool) {
	select {
	case event := <-c.send:
		return event, true
	case <-time.After(100 * time.Millisecond):
		return Event{}, false
	}
}

func TestTypingReachesOnlyTheBuddy(t *testing.T) {
	store := fakeStore{buddies: map[[2]int]bool{{1, 2}: true}}
	hub, clients := startHub(t, store, 1, 2, 3)

	if err := hub.Publish(IncomingMessage{Type: "typing", RecipientID: 2}, 1); err != nil {
		t.Fatal(err)
	}
	if err := hub.Publish(IncomingMessage{Type: "typing", RecipientID: 3}, 1); err != nil {
		t.Fatal(err)
	}

	event, ok := receive(clients[2])
	if !ok || event.Type != "typing" || event.UserID != 1 || event.RoomID != 0 {
		t.Fatalf("buddy got %+v, delivered=%v", event, ok)
	}
	if event, ok := receive(clients[3]); ok {
		t.Fatalf("non-buddy received %+v", event)
	}
	if event, ok := receive(clients[1]); ok {
		t.Fatalf("sender received %+v", event)
	}
}

func TestTypingInRoomSkipsSenderAndNonMembers(t *testing.T) {
	store := fakeStore{rooms: map[int][]int{7: {1, 2}}}
	hub, clients := startHub(t, store, 1, 2, 3)

	if err := hub.Publish(IncomingMessage{Type: "typing", RoomID: 7}, 1); err != nil {
		t.Fatal(err)
	}
	if err := hub.Publish(IncomingMessage{Type: "typing", RoomID: 7}, 3); err != nil {
		t.Fatal(err)
	}

	event, ok := receive(clients[2])
	if !ok || event.Type != "typing" || event.UserID != 1 || event.RoomID != 7 {
		t.Fatalf("member got %+v, delivered=%v", event, ok)
	}
	if _, ok := receive(clients[2]); ok {
		t.Fatal("a non-member's typing event reached the room")
	}
	if event, ok := receive(clients[1]); ok {
		t.Fatalf("sender received %+v", event)
	}
}

func TestRoomMessageIncludesRealSenderName(t *testing.T) {
	store := fakeStore{
		rooms: map[int][]int{7: {1, 2}},
		users: map[int]string{1: "alice"},
	}
	Hub, clients := startHub(t, store, 1, 2)

	if err := Hub.Publish(IncomingMessage{Type: "room", RoomID: 7, Content: "hello"}, 1); err != nil {
		t.Fatal(err)
	}

	event, ok := receive(clients[2])
	if !ok || event.Type != "message" || event.Message == nil || event.Message.SenderName != "alice" {
		t.Fatalf("room member got %+v, delivered=%v", event, ok)
	}
}
