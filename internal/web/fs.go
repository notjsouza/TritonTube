// Lab 7: Implement a local filesystem video content service

package web

import (
	"fmt"
	"os"
	"path/filepath"
)

// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct {
	baseDir string
}

// Uncomment the following line to ensure FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)

func NewFSVideoContentService(baseDir string) *FSVideoContentService {
	return &FSVideoContentService{baseDir: baseDir}
}

func (fs *FSVideoContentService) Write(videoId string, filename string, data []byte) error {
	videoDir := filepath.Join(fs.baseDir, videoId)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(videoDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write video file: %w", err)
	}

	return nil
}

func (fs *FSVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	fullPath := filepath.Join(fs.baseDir, videoId, filename)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

func (fs *FSVideoContentService) DeleteAll(videoId string) error {
	videoDir := filepath.Join(fs.baseDir, videoId)
	if err := os.RemoveAll(videoDir); err != nil {
		return fmt.Errorf("failed to delete video directory: %w", err)
	}

	return nil
}
