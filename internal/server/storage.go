package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	_ "modernc.org/sqlite" // sqlite driver
)

// Pin represents a stored pin entry.
type Pin struct {
	ID          int64
	Filename    string
	UploadedAt  time.Time
	Title       string
	Description string
	GUID        string
	PubDate     time.Time
	MimeType    string
	Data        []byte
}

// Storage wraps SQLite persistence.
type Storage struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewStorage initializes storage and ensures schema.
func NewStorage(dbPath string, logger *zap.Logger) (*Storage, error) {
	finalPath, err := prepareDBPath(dbPath)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", finalPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Storage{db: db, logger: logger}
	if err := s.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) initSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS pins (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	filename TEXT NOT NULL,
	uploaded_at DATETIME NOT NULL,
	title TEXT,
	description TEXT,
	guid TEXT NOT NULL UNIQUE,
	pub_date DATETIME NOT NULL,
	mime_type TEXT,
	data BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pub_date ON pins(pub_date DESC);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// InsertPin stores a pin and returns its ID.
func (s *Storage) InsertPin(ctx context.Context, p Pin) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
INSERT INTO pins (filename, uploaded_at, title, description, guid, pub_date, mime_type, data)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Filename, p.UploadedAt.UTC(), p.Title, p.Description, p.GUID, p.PubDate.UTC(), p.MimeType, p.Data)
	if err != nil {
		return 0, fmt.Errorf("insert pin: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// ListPins returns pins ordered by pubDate descending, limited by provided count.
func (s *Storage) ListPins(ctx context.Context, limit int) ([]Pin, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, filename, uploaded_at, title, description, guid, pub_date, mime_type, data
FROM pins
ORDER BY pub_date DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query pins: %w", err)
	}
	defer rows.Close()

	var pins []Pin
	for rows.Next() {
		var p Pin
		if err := rows.Scan(&p.ID, &p.Filename, &p.UploadedAt, &p.Title, &p.Description, &p.GUID, &p.PubDate, &p.MimeType, &p.Data); err != nil {
			return nil, fmt.Errorf("scan pin: %w", err)
		}
		pins = append(pins, p)
	}

	return pins, rows.Err()
}

// GetPinData returns mime type and raw image bytes by id.
func (s *Storage) GetPinData(ctx context.Context, id int64) (string, []byte, error) {
	row := s.db.QueryRowContext(ctx, `SELECT mime_type, data FROM pins WHERE id = ?`, id)
	var mime string
	var data []byte
	if err := row.Scan(&mime, &data); err != nil {
		if err == sql.ErrNoRows {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("scan image: %w", err)
	}
	return mime, data, nil
}

// Close releases the underlying DB connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

func prepareDBPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create db dir: %w", err)
	}
	return p, nil
}
