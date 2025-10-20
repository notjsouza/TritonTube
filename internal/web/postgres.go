package web

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type PostgresVideoMetadataService struct {
	db *sql.DB
}

// Ensure PostgresVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*PostgresVideoMetadataService)(nil)

// NewPostgresVideoMetadataService creates a new PostgreSQL metadata service
// connectionString format: postgres://user:password@host:port/database?sslmode=require
func NewPostgresVideoMetadataService(connectionString string) (*PostgresVideoMetadataService, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create the table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS video_metadata (
			video_id TEXT PRIMARY KEY,
			uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Create an index on uploaded_at for faster sorting
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_video_metadata_uploaded_at 
		ON video_metadata(uploaded_at DESC)
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return &PostgresVideoMetadataService{db: db}, nil
}

func (s *PostgresVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO video_metadata (video_id, uploaded_at) VALUES ($1, $2)",
		videoId, uploadedAt,
	)

	if err != nil {
		// PostgreSQL unique constraint error code is 23505
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
			return errors.New("video ID already exists")
		}

		return fmt.Errorf("failed to insert video metadata: %w", err)
	}

	return nil
}

func (s *PostgresVideoMetadataService) List() ([]VideoMetadata, error) {
	rows, err := s.db.Query("SELECT video_id, uploaded_at FROM video_metadata ORDER BY uploaded_at DESC")

	if err != nil {
		return nil, fmt.Errorf("failed to query video metadata: %w", err)
	}
	defer rows.Close()

	var result []VideoMetadata
	for rows.Next() {
		var v VideoMetadata
		if err := rows.Scan(&v.Id, &v.UploadedAt); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

func (s *PostgresVideoMetadataService) Read(videoId string) (*VideoMetadata, error) {
	row := s.db.QueryRow("SELECT video_id, uploaded_at FROM video_metadata WHERE video_id = $1", videoId)

	var v VideoMetadata
	if err := row.Scan(&v.Id, &v.UploadedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read video metadata: %w", err)
	}

	return &v, nil
}

func (s *PostgresVideoMetadataService) Delete(videoId string) error {
	result, err := s.db.Exec("DELETE FROM video_metadata WHERE video_id = $1", videoId)
	if err != nil {
		return fmt.Errorf("failed to delete video metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("video not found")
	}

	return nil
}

// Close closes the database connection
func (s *PostgresVideoMetadataService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
