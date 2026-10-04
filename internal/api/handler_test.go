package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/swiftahul20/personal-messenger/internal/chat"
	"github.com/swiftahul20/personal-messenger/internal/room"
	"github.com/swiftahul20/personal-messenger/internal/user"
)

type stubStore struct {
	listings    []room.Listing
	members     map[int][]room.Member
	lastQuery   string
	lastBrowser int
}

func (s *stubStore) CreateOrGet(context.Context, string) (user.User, error) { return user.User{}, nil }
func (s *stubStore) FindByUsername(context.Context, string) (user.User, error) {
	return user.User{}, nil
}
func (s *stubStore) GetByID(context.Context, int) (user.User, error)       { return user.User{}, nil }
func (s *stubStore) AddBuddy(context.Context, int, int) error              { return nil }
func (s *stubStore) ListBuddies(context.Context, int) ([]user.User, error) { return nil, nil }
func (s *stubStore) CreateRoom(context.Context, int, string) (room.Room, error) {
	return room.Room{}, nil
}
func (s *stubStore) JoinRoom(context.Context, int, int) error            { return nil }
func (s *stubStore) ListRooms(context.Context, int) ([]room.Room, error) { return nil, nil }
func (s *stubStore) SaveMessage(context.Context, *chat.Message) error    { return nil }
func (s *stubStore) AreBuddies(context.Context, int, int) (bool, error)  { return false, nil }
func (s *stubStore) RoomMemberIDs(context.Context, int) ([]int, error)   { return nil, nil }
func (s *stubStore) BuddyIDs(context.Context, int) ([]int, error)        { return nil, nil }
func (s *stubStore) DMHistory(context.Context, int, int, int64, int) ([]chat.Message, error) {
	return nil, nil
}
func (s *stubStore) RoomHistory(context.Context, int, int64, int) ([]chat.Message, error) {
	return nil, nil
}

func (s *stubStore) BrowseRooms(_ context.Context, userID int, query string) ([]room.Listing, error) {
	s.lastBrowser = userID
	s.lastQuery = query
	return s.listings, nil
}

func (s *stubStore) RoomMembers(_ context.Context, roomID int) ([]room.Member, error) {
	return s.members[roomID], nil
}

func (s *stubStore) IsRoomMember(_ context.Context, roomID, userID int) (bool, error) {
	for _, member := range s.members[roomID] {
		if member.ID == userID {
			return true, nil
		}
	}
	return false, nil
}

func newTestHandler(t *testing.T, store *stubStore) http.Handler {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hub := chat.NewHub(store)
	go hub.Run(ctx)
	return NewHandler(store, store, store, hub).Routes()
}

func get(handler http.Handler, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestBrowseRoomsReturnsListingsAndPassesTrimmedSearch(t *testing.T) {
	store := &stubStore{listings: []room.Listing{
		{Room: room.Room{ID: 3, Name: "after-hours"}, MemberCount: 2, Joined: true},
	}}
	handler := newTestHandler(t, store)

	response := get(handler, "/api/rooms?user_id=7&q=%20after%20")
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if store.lastBrowser != 7 || store.lastQuery != "after" {
		t.Fatalf("store got user %d query %q", store.lastBrowser, store.lastQuery)
	}
	var listings []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		MemberCount int    `json:"member_count"`
		Joined      bool   `json:"joined"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 || listings[0].ID != 3 || listings[0].MemberCount != 2 || !listings[0].Joined {
		t.Fatalf("unexpected listings: %+v", listings)
	}
}

func TestBrowseRoomsRejectsBadInput(t *testing.T) {
	handler := newTestHandler(t, &stubStore{})

	if code := get(handler, "/api/rooms").Code; code != http.StatusBadRequest {
		t.Fatalf("missing user_id returned %d", code)
	}
	long := strings.Repeat("a", roomSearchMaxLength+1)
	if code := get(handler, "/api/rooms?user_id=1&q="+long).Code; code != http.StatusBadRequest {
		t.Fatalf("over-long search returned %d", code)
	}
}

func TestRoomMembersAreOnlyVisibleToMembers(t *testing.T) {
	store := &stubStore{members: map[int][]room.Member{
		5: {{ID: 1, Username: "alice"}, {ID: 2, Username: "bob"}},
	}}
	handler := newTestHandler(t, store)

	if code := get(handler, "/api/rooms/5/members?user_id=9").Code; code != http.StatusForbidden {
		t.Fatalf("non-member returned %d", code)
	}

	response := get(handler, "/api/rooms/5/members?user_id=1")
	if response.Code != http.StatusOK {
		t.Fatalf("member returned %d: %s", response.Code, response.Body)
	}
	var members []room.Member
	if err := json.Unmarshal(response.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 || members[0].Username != "alice" || members[0].Online {
		t.Fatalf("unexpected members: %+v", members)
	}
}
