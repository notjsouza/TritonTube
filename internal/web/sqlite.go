// Lab 7: Implement a SQLite video metadata service

package web

import (
	"database/sql"
	"errors"
	"time"

	"github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct {
	db *sql.DB
}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)

func NewSQLiteVideoMetadataService(dbPath string) (*SQLiteVideoMetadataService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS video_metadata (
			video_id TEXT PRIMARY KEY,
			uploaded_at DATETIME
		)
	`)

	if err != nil {
		return nil, err
	}

	return &SQLiteVideoMetadataService{db: db}, nil
}

func (s *SQLiteVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO video_metadata (video_id, uploaded_at) VALUES (?, ?)",
		videoId, uploadedAt,
	)

	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return errors.New("video ID already exists")
		}

		return err
	}

	return nil
}

func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	rows, err := s.db.Query("SELECT video_id, uploaded_at FROM video_metadata ORDER BY uploaded_at DESC")

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VideoMetadata
	for rows.Next() {
		var v VideoMetadata
		if err := rows.Scan(&v.Id, &v.UploadedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}

	return result, nil
}

func (s *SQLiteVideoMetadataService) Read(videoId string) (*VideoMetadata, error) {
	row := s.db.QueryRow("SELECT video_id, uploaded_at FROM video_metadata WHERE video_id = ?", videoId)

	var v VideoMetadata
	if err := row.Scan(&v.Id, &v.UploadedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &v, nil
}
