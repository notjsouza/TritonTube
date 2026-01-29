package web

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBVideoMetadataService implements VideoMetadataService using DynamoDB
type DynamoDBVideoMetadataService struct {
	client    *dynamodb.Client
	tableName string
}

// videoMetadataItem represents the DynamoDB item structure
type videoMetadataItem struct {
	ID         string `dynamodbav:"id"`
	UploadedAt int64  `dynamodbav:"uploadedAt"` // Unix timestamp
}

// NewDynamoDBVideoMetadataService creates a new DynamoDB metadata service
// tableName should be the DynamoDB table name
func NewDynamoDBVideoMetadataService(tableName string) (*DynamoDBVideoMetadataService, error) {
	if tableName == "" {
		return nil, fmt.Errorf("DynamoDB table name cannot be empty")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	return &DynamoDBVideoMetadataService{
		client:    client,
		tableName: tableName,
	}, nil
}

// Create adds a new video metadata entry
func (s *DynamoDBVideoMetadataService) Create(id string, uploadedAt time.Time) error {
	item := videoMetadataItem{
		ID:         id,
		UploadedAt: uploadedAt.Unix(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = s.client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// Read retrieves video metadata by ID
func (s *DynamoDBVideoMetadataService) Read(id string) (*VideoMetadata, error) {
	result, err := s.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if result.Item == nil {
		return nil, nil // Not found
	}

	var item videoMetadataItem
	err = attributevalue.UnmarshalMap(result.Item, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return &VideoMetadata{
		Id:         item.ID,
		UploadedAt: time.Unix(item.UploadedAt, 0),
	}, nil
}

// List retrieves all video metadata entries
func (s *DynamoDBVideoMetadataService) List() ([]VideoMetadata, error) {
	result, err := s.client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan table: %w", err)
	}

	var items []videoMetadataItem
	err = attributevalue.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal items: %w", err)
	}

	videos := make([]VideoMetadata, 0, len(items))
	for _, item := range items {
		videos = append(videos, VideoMetadata{
			Id:         item.ID,
			UploadedAt: time.Unix(item.UploadedAt, 0),
		})
	}

	return videos, nil
}

// Delete removes video metadata by ID
func (s *DynamoDBVideoMetadataService) Delete(id string) error {
	_, err := s.client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}
