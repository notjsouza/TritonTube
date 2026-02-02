package web

import "time"

type VideoMetadata struct {
	Id         string
	UploadedAt time.Time
	Status     string // "processing", "ready", "error"
}

type VideoMetadataService interface {
	Read(id string) (*VideoMetadata, error)
	List() ([]VideoMetadata, error)
	Create(videoId string, uploadedAt time.Time) error
	CreateWithStatus(videoId string, uploadedAt time.Time, status string) error
	UpdateStatus(videoId string, status string) error
	Delete(id string) error
}

type VideoContentService interface {
	Read(videoId string, filename string) ([]byte, error)
	Write(videoId string, filename string, data []byte) error
	DeleteAll(videoId string) error
}
