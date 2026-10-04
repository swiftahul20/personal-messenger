package data

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swiftahul20/personal-messenger/internal/chat"
	"github.com/swiftahul20/personal-messenger/internal/room"
	"github.com/swiftahul20/personal-messenger/internal/user"
)

//go:embed schema.sql
var schema embed.FS

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Migrate(ctx context.Context) error {
	ddl, err := schema.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, string(ddl))
	return err
}

func (p *Postgres) CreateOrGet(ctx context.Context, username string) (user.User, error) {
	var result user.User
	err := p.pool.QueryRow(ctx, `
		INSERT INTO users (username) VALUES ($1)
		ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
		RETURNING id, username, created_at`, username).Scan(&result.ID, &result.Username, &result.CreatedAt)
	return result, err
}

func (p *Postgres) FindByUsername(ctx context.Context, username string) (user.User, error) {
	var result user.User
	err := p.pool.QueryRow(ctx, `SELECT id, username, created_at FROM users WHERE username = $1`, username).
		Scan(&result.ID, &result.Username, &result.CreatedAt)
	return result, err
}

func (p *Postgres) GetByID(ctx context.Context, id int) (user.User, error) {
	var result user.User
	err := p.pool.QueryRow(ctx, `SELECT id, username, created_at FROM users WHERE id = $1`, id).
		Scan(&result.ID, &result.Username, &result.CreatedAt)
	return result, err
}

func (p *Postgres) AddBuddy(ctx context.Context, userID, buddyID int) error {
	if userID == buddyID {
		return errors.New("cannot add yourself as a buddy")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO user_buddies (user_id, buddy_id) VALUES ($1, $2), ($2, $1)
		ON CONFLICT DO NOTHING`, userID, buddyID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ListBuddies(ctx context.Context, userID int) ([]user.User, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT u.id, u.username, u.created_at
		FROM user_buddies b JOIN users u ON u.id = b.buddy_id
		WHERE b.user_id = $1 ORDER BY u.username`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buddies := make([]user.User, 0)
	for rows.Next() {
		var buddy user.User
		if err := rows.Scan(&buddy.ID, &buddy.Username, &buddy.CreatedAt); err != nil {
			return nil, err
		}
		buddies = append(buddies, buddy)
	}
	return buddies, rows.Err()
}

func (p *Postgres) CreateRoom(ctx context.Context, userID int, name string) (room.Room, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return room.Room{}, err
	}
	defer tx.Rollback(ctx)
	var result room.Room
	if err := tx.QueryRow(ctx, `INSERT INTO rooms (name) VALUES ($1) RETURNING id, name, created_at`, name).
		Scan(&result.ID, &result.Name, &result.CreatedAt); err != nil {
		return room.Room{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO room_members (room_id, user_id) VALUES ($1, $2)`, result.ID, userID); err != nil {
		return room.Room{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return room.Room{}, err
	}
	return result, nil
}

func (p *Postgres) JoinRoom(ctx context.Context, roomID, userID int) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roomID, userID)
	return err
}

func (p *Postgres) ListRooms(ctx context.Context, userID int) ([]room.Room, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT r.id, r.name, r.created_at
		FROM room_members m JOIN rooms r ON r.id = m.room_id
		WHERE m.user_id = $1 ORDER BY r.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := make([]room.Room, 0)
	for rows.Next() {
		var item room.Room
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, item)
	}
	return rooms, rows.Err()
}

func (p *Postgres) BrowseRooms(ctx context.Context, userID int, query string) ([]room.Listing, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT r.id, r.name, r.created_at,
			(SELECT count(*) FROM room_members m WHERE m.room_id = r.id) AS member_count,
			EXISTS (SELECT 1 FROM room_members m WHERE m.room_id = r.id AND m.user_id = $1) AS joined
		FROM rooms r
		WHERE $2::text = '' OR position(lower($2::text) in lower(r.name)) > 0
		ORDER BY member_count DESC, r.name
		LIMIT 50`, userID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	listings := make([]room.Listing, 0)
	for rows.Next() {
		var item room.Listing
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt, &item.MemberCount, &item.Joined); err != nil {
			return nil, err
		}
		listings = append(listings, item)
	}
	return listings, rows.Err()
}

func (p *Postgres) RoomMembers(ctx context.Context, roomID int) ([]room.Member, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT u.id, u.username
		FROM room_members m JOIN users u ON u.id = m.user_id
		WHERE m.room_id = $1 ORDER BY u.username`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]room.Member, 0)
	for rows.Next() {
		var member room.Member
		if err := rows.Scan(&member.ID, &member.Username); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (p *Postgres) SaveMessage(ctx context.Context, message *chat.Message) error {
	return p.pool.QueryRow(ctx, `
		WITH saved AS (
			INSERT INTO messages (sender_id, recipient_id, room_id, content)
			VALUES ($1, $2, $3, $4)
			RETURNING id, sender_id, sent_at
		)
		SELECT saved.id, saved.sent_at, sender.username
		FROM saved JOIN users sender ON sender.id = saved.sender_id`,
		message.SenderID, message.RecipientID, message.RoomID, message.Content).
		Scan(&message.ID, &message.SentAt, &message.SenderName)
}

func (p *Postgres) AreBuddies(ctx context.Context, userID, buddyID int) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_buddies WHERE user_id = $1 AND buddy_id = $2)`, userID, buddyID).Scan(&exists)
	return exists, err
}

func (p *Postgres) IsRoomMember(ctx context.Context, roomID, userID int) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`, roomID, userID).Scan(&exists)
	return exists, err
}

func (p *Postgres) RoomMemberIDs(ctx context.Context, roomID int) ([]int, error) {
	rows, err := p.pool.Query(ctx, `SELECT user_id FROM room_members WHERE room_id = $1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (p *Postgres) BuddyIDs(ctx context.Context, userID int) ([]int, error) {
	rows, err := p.pool.Query(ctx, `SELECT buddy_id FROM user_buddies WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (p *Postgres) DMHistory(ctx context.Context, userID, buddyID int, beforeID int64, limit int) ([]chat.Message, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT history.id, history.sender_id, history.recipient_id, history.room_id,
			history.content, history.sent_at, sender.username
		FROM (
			SELECT id, sender_id, recipient_id, room_id, content, sent_at FROM messages
			WHERE room_id IS NULL AND ((sender_id = $1 AND recipient_id = $2) OR (sender_id = $2 AND recipient_id = $1))
				AND ($3 = 0 OR id < $3)
			ORDER BY id DESC LIMIT $4
		) history
		JOIN users sender ON sender.id = history.sender_id
		ORDER BY history.id ASC`, userID, buddyID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

func (p *Postgres) RoomHistory(ctx context.Context, roomID int, beforeID int64, limit int) ([]chat.Message, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT history.id, history.sender_id, history.recipient_id, history.room_id,
			history.content, history.sent_at, sender.username
		FROM (
			SELECT id, sender_id, recipient_id, room_id, content, sent_at FROM messages
			WHERE room_id = $1 AND ($2 = 0 OR id < $2)
			ORDER BY id DESC LIMIT $3
		) history
		JOIN users sender ON sender.id = history.sender_id
		ORDER BY history.id ASC`, roomID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

func scanMessages(rows pgx.Rows) ([]chat.Message, error) {
	defer rows.Close()
	messages := make([]chat.Message, 0)
	for rows.Next() {
		var message chat.Message
		if err := rows.Scan(&message.ID, &message.SenderID, &message.RecipientID, &message.RoomID, &message.Content, &message.SentAt, &message.SenderName); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

var _ user.Store = (*Postgres)(nil)
var _ room.Store = (*Postgres)(nil)
var _ chat.Store = (*Postgres)(nil)
