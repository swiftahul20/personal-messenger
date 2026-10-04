package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/swiftahul20/personal-messenger/internal/chat"
	"github.com/swiftahul20/personal-messenger/internal/room"
	"github.com/swiftahul20/personal-messenger/internal/user"
)

type Handler struct {
	users  user.Store
	rooms  room.Store
	chat   chat.Store
	hub    *chat.Hub
	upgrad websocket.Upgrader
}

func NewHandler(users user.Store, rooms room.Store, messages chat.Store, hub *chat.Hub) *Handler {
	return &Handler{
		users: users, rooms: rooms, chat: messages, hub: hub,
		upgrad: websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024},
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/users", h.createUser)
	mux.HandleFunc("GET /api/users/{userID}/buddies", h.listBuddies)
	mux.HandleFunc("POST /api/users/{userID}/buddies", h.addBuddy)
	mux.HandleFunc("GET /api/users/{userID}/rooms", h.listRooms)
	mux.HandleFunc("POST /api/rooms", h.createRoom)
	mux.HandleFunc("GET /api/rooms", h.browseRooms)
	mux.HandleFunc("GET /api/rooms/{roomID}/members", h.roomMembers)
	mux.HandleFunc("POST /api/rooms/{roomID}/join", h.joinRoom)
	mux.HandleFunc("GET /api/messages/dm", h.dmHistory)
	mux.HandleFunc("GET /api/messages/room", h.roomHistory)
	mux.HandleFunc("GET /ws", h.websocket)
	return mux
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := user.ValidateUsername(input.Username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.users.CreateOrGet(r.Context(), input.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) listBuddies(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	buddies, err := h.users.ListBuddies(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list buddies")
		return
	}
	presence := h.hub.OnlineUsers()
	result := make([]user.Buddy, 0, len(buddies))
	for _, buddy := range buddies {
		result = append(result, user.Buddy{User: buddy, Online: presence[buddy.ID]})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) addBuddy(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	var input struct {
		Username string `json:"username"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	buddy, err := h.users.FindByUsername(r.Context(), input.Username)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err := h.users.AddBuddy(r.Context(), userID, buddy.ID); err != nil {
		writeError(w, http.StatusBadRequest, "could not add buddy")
		return
	}
	writeJSON(w, http.StatusCreated, buddy)
}

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID int    `json:"user_id"`
		Name   string `json:"name"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.UserID <= 0 || room.ValidateName(input.Name) != nil {
		writeError(w, http.StatusBadRequest, "user_id and a room name of 1-100 characters are required")
		return
	}
	result, err := h.rooms.CreateRoom(r.Context(), input.UserID, input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not create room")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) joinRoom(w http.ResponseWriter, r *http.Request) {
	roomID, ok := pathID(w, r, "roomID")
	if !ok {
		return
	}
	var input struct {
		UserID int `json:"user_id"`
	}
	if decodeJSON(w, r, &input) != nil || input.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "a positive user_id is required")
		return
	}
	if err := h.rooms.JoinRoom(r.Context(), roomID, input.UserID); err != nil {
		writeError(w, http.StatusBadRequest, "could not join room")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(w, r, "userID")
	if !ok {
		return
	}
	result, err := h.rooms.ListRooms(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list rooms")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

const roomSearchMaxLength = 100

func (h *Handler) browseRooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := queryID(w, r, "user_id")
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > roomSearchMaxLength {
		writeError(w, http.StatusBadRequest, "search text must be 100 characters or fewer")
		return
	}
	result, err := h.rooms.BrowseRooms(r.Context(), userID, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search rooms")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) roomMembers(w http.ResponseWriter, r *http.Request) {
	roomID, ok := pathID(w, r, "roomID")
	if !ok {
		return
	}
	userID, ok := queryID(w, r, "user_id")
	if !ok {
		return
	}
	member, err := h.chat.IsRoomMember(r.Context(), roomID, userID)
	if err != nil || !member {
		writeError(w, http.StatusForbidden, "user is not a member of this room")
		return
	}
	members, err := h.rooms.RoomMembers(r.Context(), roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list room members")
		return
	}
	presence := h.hub.OnlineUsers()
	for i := range members {
		members[i].Online = presence[members[i].ID]
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) dmHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := queryID(w, r, "user_id")
	if !ok {
		return
	}
	buddyID, ok := queryID(w, r, "with")
	if !ok {
		return
	}
	allowed, err := h.chat.AreBuddies(r.Context(), userID, buddyID)
	if err != nil || !allowed {
		writeError(w, http.StatusForbidden, "users are not buddies")
		return
	}
	beforeID, ok := queryBeforeID(w, r)
	if !ok {
		return
	}
	messages, err := h.chat.DMHistory(r.Context(), userID, buddyID, beforeID, queryLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load message history")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *Handler) roomHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := queryID(w, r, "user_id")
	if !ok {
		return
	}
	roomID, ok := queryID(w, r, "room_id")
	if !ok {
		return
	}
	member, err := h.chat.IsRoomMember(r.Context(), roomID, userID)
	if err != nil || !member {
		writeError(w, http.StatusForbidden, "user is not a member of this room")
		return
	}
	beforeID, ok := queryBeforeID(w, r)
	if !ok {
		return
	}
	messages, err := h.chat.RoomHistory(r.Context(), roomID, beforeID, queryLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load message history")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *Handler) websocket(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(r.URL.Query().Get("user_id"))
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "a positive user_id query parameter is required")
		return
	}
	if _, err := h.users.GetByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	conn, err := h.upgrad.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	client := chat.NewClient(userID, h.hub, conn)
	go client.Run()
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(r.PathValue(name))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid identifier")
		return 0, false
	}
	return id, true
}

func queryID(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func queryBeforeID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.URL.Query().Get("before_id")
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "before_id must be a positive message ID")
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}
