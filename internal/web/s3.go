package web

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3VideoContentService implements VideoContentService using AWS S3
type S3VideoContentService struct {
	client     *s3.Client
	bucketName string
}

// NewS3VideoContentService creates a new S3-backed video content service
func NewS3VideoContentService(bucketName string) (*S3VideoContentService, error) {
	// Load AWS configuration from environment
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	return &S3VideoContentService{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// Write uploads a file to S3
func (s *S3VideoContentService) Write(videoId, filename string, data []byte) error {
	key := fmt.Sprintf("%s/%s", videoId, filename)

	// Determine content type
	contentType := "application/octet-stream"
	if filename == "manifest.mpd" {
		contentType = "application/dash+xml"
	} else if filename == "thumbnail.jpg" {
		contentType = "image/jpeg"
	} else if len(filename) > 4 && filename[len(filename)-4:] == ".m4s" {
		contentType = "video/iso.segment"
	}

	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	log.Printf("Uploaded %s to S3: s3://%s/%s", filename, s.bucketName, key)
	return nil
}

// Read downloads a file from S3
func (s *S3VideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)

	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	// Read the entire object into memory
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object: %w", err)
	}

	return buf.Bytes(), nil
}

// DeleteAll removes a video and all its files from S3
func (s *S3VideoContentService) DeleteAll(videoId string) error {
	// List all objects with the videoId prefix
	listResult, err := s.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(videoId + "/"),
	})

	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %w", err)
	}

	// Delete each object
	for _, obj := range listResult.Contents {
		_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    obj.Key,
		})

		if err != nil {
			log.Printf("Warning: failed to delete %s: %v", *obj.Key, err)
		} else {
			log.Printf("Deleted from S3: s3://%s/%s", s.bucketName, *obj.Key)
		}
	}

	return nil
}

// GetBucketName returns the S3 bucket name (useful for generating URLs)
func (s *S3VideoContentService) GetBucketName() string {
	return s.bucketName
}

// GetS3BucketFromEnv gets the S3 bucket name from environment variable
func GetS3BucketFromEnv() string {
	bucket := os.Getenv("S3_BUCKET_NAME")
	if bucket == "" {
		bucket = "tritontube-video-content" // default fallback
	}
	return bucket
}
