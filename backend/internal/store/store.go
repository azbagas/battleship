package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// GameRecord captures the immutable summary record of a completed match.
type GameRecord struct {
	ID          string    `json:"id"`
	StartedAt   time.Time `json:"startedAt"`
	EndedAt     time.Time `json:"endedAt"`
	Player1Name string    `json:"player1Name"`
	Player2Name string    `json:"player2Name"`
	WinnerSlot  *int      `json:"winnerSlot,omitempty"` // 1 (host), 2 (guest), or nil (abandoned)
	EndReason   string    `json:"endReason"`           // "all_sunk", "forfeit", "disconnect_timeout", "abandoned"
	TotalShots  int       `json:"totalShots"`
	ShotLog     string    `json:"shotLog"`              // JSON serialized array of shot records
}

// Store defines persistence operations for completed matches.
type Store interface {
	SaveGame(ctx context.Context, record *GameRecord) error
	GetGame(ctx context.Context, id string) (*GameRecord, error)
	Close() error
}

// SQLiteStore implements Store backed by modernc.org/sqlite without CGO.
type SQLiteStore struct {
	db *sql.DB
}

// New constructs a SQLiteStore, sets pragmas, and migrates the games table schema.
func New(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %q: %w", dbPath, err)
	}

	// Pragmas for durability and concurrent access:
	// WAL mode allows concurrent readers and writer; busy_timeout waits up to 5s on lock.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite pragmas: %w", err)
	}

	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize sqlite schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY,
		started_at TIMESTAMP NOT NULL,
		ended_at TIMESTAMP NOT NULL,
		player1_name TEXT NOT NULL,
		player2_name TEXT NOT NULL,
		winner_slot INTEGER,
		end_reason TEXT NOT NULL,
		total_shots INTEGER NOT NULL,
		shot_log TEXT NOT NULL
	);`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("create games table: %w", err)
	}
	return nil
}

// SaveGame inserts a single finished game record into the games table.
func (s *SQLiteStore) SaveGame(ctx context.Context, record *GameRecord) error {
	if record == nil {
		return fmt.Errorf("game record cannot be nil")
	}
	if record.ID == "" {
		return fmt.Errorf("game id cannot be empty")
	}

	query := `
	INSERT INTO games (id, started_at, ended_at, player1_name, player2_name, winner_slot, end_reason, total_shots, shot_log)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.db.ExecContext(ctx, query,
		record.ID,
		record.StartedAt.UTC(),
		record.EndedAt.UTC(),
		record.Player1Name,
		record.Player2Name,
		record.WinnerSlot,
		record.EndReason,
		record.TotalShots,
		record.ShotLog,
	)
	if err != nil {
		return fmt.Errorf("insert game record %q: %w", record.ID, err)
	}
	return nil
}

// GetGame retrieves a completed match record by game ID.
func (s *SQLiteStore) GetGame(ctx context.Context, id string) (*GameRecord, error) {
	query := `
	SELECT id, started_at, ended_at, player1_name, player2_name, winner_slot, end_reason, total_shots, shot_log
	FROM games
	WHERE id = ?;
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var (
		rec        GameRecord
		winnerSlot sql.NullInt64
		startedAt  time.Time
		endedAt    time.Time
	)

	err := row.Scan(
		&rec.ID,
		&startedAt,
		&endedAt,
		&rec.Player1Name,
		&rec.Player2Name,
		&winnerSlot,
		&rec.EndReason,
		&rec.TotalShots,
		&rec.ShotLog,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game %q not found: %w", id, err)
		}
		return nil, fmt.Errorf("scan game record %q: %w", id, err)
	}

	rec.StartedAt = startedAt.UTC()
	rec.EndedAt = endedAt.UTC()
	if winnerSlot.Valid {
		ws := int(winnerSlot.Int64)
		rec.WinnerSlot = &ws
	}

	return &rec, nil
}

// Close terminates the underlying database connection pool.
func (s *SQLiteStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

