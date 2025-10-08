# TritonTube AWS Deployment Guide
## Architecture: S3 + CloudFront + ECS/Fargate

This guide walks you through deploying TritonTube to AWS using a serverless storage approach with S3 and CloudFront for video delivery, eliminating the need for storage node EC2 instances.

---

## 📐 Architecture Overview

```
┌─────────────┐
│   Users     │
└──────┬──────┘
       │
       ├─────────────────────────────────────────┐
       │                                         │
       ▼                                         ▼
┌──────────────────┐                   ┌──────────────────┐
│   CloudFront     │                   │   CloudFront     │
│  (Frontend CDN)  │                   │  (Video CDN)     │
└────────┬─────────┘                   └────────┬─────────┘
         │                                      │
         ▼                                      ▼
┌──────────────────┐                   ┌──────────────────┐
│   S3 Bucket      │                   │   S3 Bucket      │
│ (React Static)   │                   │ (Video Content)  │
└──────────────────┘                   └──────────────────┘
                                                │
       ┌────────────────────────────────────────┘
       │
       ▼
┌──────────────────┐
│  ALB (API)       │
└────────┬─────────┘
         │
    ┌────┴─────┬─────────┐
    ▼          ▼         ▼
┌────────┐ ┌────────┐ ┌────────┐
│  ECS   │ │  ECS   │ │  ECS   │
│ Task 1 │ │ Task 2 │ │ Task 3 │
└───┬────┘ └───┬────┘ └───┬────┘
    │          │          │
    └──────────┴──────────┘
               │
      ┌────────┴────────┬──────────────┬──────────────┐
      ▼                 ▼              ▼              ▼
┌──────────┐   ┌──────────────┐  ┌────────┐   ┌──────────┐
│   RDS    │   │ ElastiCache  │  │  SQS   │   │  Lambda  │
│(Postgres)│   │   (Redis)    │  │ Queue  │   │ (FFmpeg) │
└──────────┘   └──────────────┘  └────────┘   └─────┬────┘
                                                     │
                                                     ▼
                                            ┌──────────────┐
                                            │  S3 Bucket   │
                                            │  (Uploads)   │
                                            └──────────────┘
```

---

## 🎯 Prerequisites

1. **AWS Account** with appropriate permissions
2. **AWS CLI** installed and configured (`aws configure`)
3. **Docker** installed (for building container images)
4. **Node.js** and **npm** (for building React frontend)
5. **Go 1.24+** (for building backend)
6. **Domain name** (optional, for custom domain)

---

## 📦 Phase 1: S3 Buckets Setup

### 1.1 Create S3 Buckets

```bash
# Set your project name
export PROJECT_NAME="tritontube"
export AWS_REGION="us-west-1

# Create bucket for video content
aws s3 mb s3://${PROJECT_NAME}-video-content --region ${AWS_REGION}

# Create bucket for frontend static files
aws s3 mb s3://${PROJECT_NAME}-frontend --region ${AWS_REGION}

# Create bucket for original uploads (temporary storage)
aws s3 mb s3://${PROJECT_NAME}-uploads --region ${AWS_REGION}
```

### 1.2 Configure S3 Bucket Policies

**Video Content Bucket Policy** (allow CloudFront access):
```bash
cat > video-bucket-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowCloudFrontAccess",
      "Effect": "Allow",
      "Principal": {
        "Service": "cloudfront.amazonaws.com"
      },
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::${PROJECT_NAME}-video-content/*"
    }
  ]
}
EOF

aws s3api put-bucket-policy \
  --bucket ${PROJECT_NAME}-video-content \
  --policy file://video-bucket-policy.json
```

**Frontend Bucket Policy** (public read):
```bash
cat > frontend-bucket-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::${PROJECT_NAME}-frontend/*"
    }
  ]
}
EOF

aws s3api put-bucket-policy \
  --bucket ${PROJECT_NAME}-frontend \
  --policy file://frontend-bucket-policy.json
```

### 1.3 Enable S3 Static Website Hosting (Frontend)

```bash
aws s3 website s3://${PROJECT_NAME}-frontend \
  --index-document index.html \
  --error-document index.html
```

### 1.4 Configure CORS for Video Bucket

```bash
cat > cors-config.json <<EOF
{
  "CORSRules": [
    {
      "AllowedOrigins": ["*"],
      "AllowedMethods": ["GET", "HEAD"],
      "AllowedHeaders": ["*"],
      "MaxAgeSeconds": 3600
    }
  ]
}
EOF

aws s3api put-bucket-cors \
  --bucket ${PROJECT_NAME}-video-content \
  --cors-configuration file://cors-config.json
```

---

## 🌐 Phase 2: CloudFront CDN Setup

### 2.1 Create CloudFront Distribution for Video Content

```bash
cat > cloudfront-video-config.json <<EOF
{
  "CallerReference": "tritontube-video-$(date +%s)",
  "Comment": "TritonTube Video Content CDN",
  "Enabled": true,
  "Origins": {
    "Quantity": 1,
    "Items": [
      {
        "Id": "S3-${PROJECT_NAME}-video-content",
        "DomainName": "${PROJECT_NAME}-video-content.s3.${AWS_REGION}.amazonaws.com",
        "S3OriginConfig": {
          "OriginAccessIdentity": ""
        }
      }
    ]
  },
  "DefaultCacheBehavior": {
    "TargetOriginId": "S3-${PROJECT_NAME}-video-content",
    "ViewerProtocolPolicy": "redirect-to-https",
    "AllowedMethods": {
      "Quantity": 2,
      "Items": ["GET", "HEAD"],
      "CachedMethods": {
        "Quantity": 2,
        "Items": ["GET", "HEAD"]
      }
    },
    "ForwardedValues": {
      "QueryString": false,
      "Cookies": {
        "Forward": "none"
      }
    },
    "MinTTL": 0,
    "DefaultTTL": 86400,
    "MaxTTL": 31536000,
    "Compress": true
  }
}
EOF

aws cloudfront create-distribution \
  --distribution-config file://cloudfront-video-config.json
```

**Note:** Save the CloudFront domain name from the output (e.g., `d111111abcdef8.cloudfront.net`)

### 2.2 Create CloudFront Distribution for Frontend

```bash
aws cloudfront create-distribution \
  --origin-domain-name ${PROJECT_NAME}-frontend.s3-website-${AWS_REGION}.amazonaws.com \
  --default-root-object index.html
```

---

## 🗄️ Phase 3: Database Setup (RDS)

### 3.1 Create RDS PostgreSQL Instance

```bash
# Create DB subnet group
aws rds create-db-subnet-group \
  --db-subnet-group-name ${PROJECT_NAME}-db-subnet \
  --db-subnet-group-description "TritonTube DB Subnet Group" \
  --subnet-ids subnet-xxxxx subnet-yyyyy  # Replace with your subnet IDs

# Create security group
aws ec2 create-security-group \
  --group-name ${PROJECT_NAME}-db-sg \
  --description "TritonTube RDS Security Group" \
  --vpc-id vpc-xxxxx  # Replace with your VPC ID

# Allow PostgreSQL access from ECS tasks
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxxxx \  # Replace with security group ID
  --protocol tcp \
  --port 5432 \
  --source-group sg-yyyyy  # ECS security group

# Create RDS instance
aws rds create-db-instance \
  --db-instance-identifier ${PROJECT_NAME}-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version 15.3 \
  --master-username admin \
  --master-user-password 'YourStrongPassword123!' \
  --allocated-storage 20 \
  --db-subnet-group-name ${PROJECT_NAME}-db-subnet \
  --vpc-security-group-ids sg-xxxxx \
  --backup-retention-period 7 \
  --no-publicly-accessible
```

**Wait for RDS to be available** (~5-10 minutes):
```bash
aws rds wait db-instance-available --db-instance-identifier ${PROJECT_NAME}-db
```

### 3.2 Create Database Schema

Connect to your RDS instance and run the migration SQL:

```sql
-- Connect: psql -h <rds-endpoint> -U admin -d postgres

CREATE DATABASE tritontube;

\c tritontube

CREATE TABLE videos (
    id VARCHAR(255) PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    uploader VARCHAR(255),
    upload_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration INTEGER,
    views INTEGER DEFAULT 0,
    thumbnail_url VARCHAR(1000),
    manifest_url VARCHAR(1000),
    storage_path VARCHAR(1000)
);

CREATE INDEX idx_upload_date ON videos(upload_date DESC);
CREATE INDEX idx_views ON videos(views DESC);
```

---

## 🚀 Phase 4: Backend Refactoring

### 4.1 Update Go Dependencies

Add AWS SDK to `go.mod`:

```bash
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/aws-sdk-go-v2/service/sqs
go get github.com/lib/pq  # PostgreSQL driver
```

### 4.2 Create S3 Storage Service

Create `internal/web/s3storage.go`:

```go
package web

import (
    "context"
    "fmt"
    "io"
    "path/filepath"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3ContentService struct {
    client     *s3.Client
    bucketName string
    cdnDomain  string
}

func NewS3ContentService(bucketName, cdnDomain string) (*S3ContentService, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        return nil, err
    }

    client := s3.NewFromConfig(cfg)

    return &S3ContentService{
        client:     client,
        bucketName: bucketName,
        cdnDomain:  cdnDomain,
    }, nil
}

func (s *S3ContentService) Store(ctx context.Context, videoID string, filename string, data io.Reader) error {
    key := filepath.Join(videoID, filename)

    _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
        Body:   data,
    })

    return err
}

func (s *S3ContentService) GetURL(videoID string, filename string) string {
    key := filepath.Join(videoID, filename)
    return fmt.Sprintf("https://%s/%s", s.cdnDomain, key)
}

func (s *S3ContentService) Delete(ctx context.Context, videoID string) error {
    // List all objects with the video ID prefix
    listOutput, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket: aws.String(s.bucketName),
        Prefix: aws.String(videoID + "/"),
    })
    if err != nil {
        return err
    }

    // Delete all objects
    for _, obj := range listOutput.Contents {
        _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
            Bucket: aws.String(s.bucketName),
            Key:    obj.Key,
        })
        if err != nil {
            return err
        }
    }

    return nil
}
```

### 4.3 Update PostgreSQL Metadata Service

Create `internal/web/postgres.go`:

```go
package web

import (
    "database/sql"
    "fmt"

    _ "github.com/lib/pq"
)

type PostgresMetadataService struct {
    db *sql.DB
}

func NewPostgresMetadataService(connectionString string) (*PostgresMetadataService, error) {
    db, err := sql.Open("postgres", connectionString)
    if err != nil {
        return nil, err
    }

    if err := db.Ping(); err != nil {
        return nil, err
    }

    return &PostgresMetadataService{db: db}, nil
}

func (p *PostgresMetadataService) GetAllVideos() ([]Video, error) {
    rows, err := p.db.Query(`
        SELECT id, title, description, uploader, upload_date, duration, views, thumbnail_url
        FROM videos
        ORDER BY upload_date DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var videos []Video
    for rows.Next() {
        var v Video
        err := rows.Scan(&v.ID, &v.Title, &v.Description, &v.Uploader, &v.UploadDate, &v.Duration, &v.Views, &v.ThumbnailURL)
        if err != nil {
            return nil, err
        }
        videos = append(videos, v)
    }

    return videos, nil
}

func (p *PostgresMetadataService) GetVideo(id string) (*Video, error) {
    var v Video
    err := p.db.QueryRow(`
        SELECT id, title, description, uploader, upload_date, duration, views, thumbnail_url, manifest_url
        FROM videos
        WHERE id = $1
    `, id).Scan(&v.ID, &v.Title, &v.Description, &v.Uploader, &v.UploadDate, &v.Duration, &v.Views, &v.ThumbnailURL, &v.ManifestURL)

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("video not found")
    }
    if err != nil {
        return nil, err
    }

    return &v, nil
}

func (p *PostgresMetadataService) CreateVideo(v *Video) error {
    _, err := p.db.Exec(`
        INSERT INTO videos (id, title, description, uploader, upload_date, duration, views, thumbnail_url, manifest_url, storage_path)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `, v.ID, v.Title, v.Description, v.Uploader, v.UploadDate, v.Duration, v.Views, v.ThumbnailURL, v.ManifestURL, v.StoragePath)

    return err
}

func (p *PostgresMetadataService) DeleteVideo(id string) error {
    _, err := p.db.Exec("DELETE FROM videos WHERE id = $1", id)
    return err
}

func (p *PostgresMetadataService) IncrementViews(id string) error {
    _, err := p.db.Exec("UPDATE videos SET views = views + 1 WHERE id = $1", id)
    return err
}
```

### 4.4 Update Main Server

Modify `cmd/web/main.go` to support new services:

```go
// Add new flags
s3Bucket := flag.String("s3-bucket", "", "S3 bucket name for video storage")
cdnDomain := flag.String("cdn-domain", "", "CloudFront CDN domain")
dbConnString := flag.String("db", "", "PostgreSQL connection string")

// Initialize services
var metadataService web.MetadataService
var contentService web.ContentService

if *dbConnString != "" {
    metadataService, err = web.NewPostgresMetadataService(*dbConnString)
    if err != nil {
        log.Fatal(err)
    }
}

if *s3Bucket != "" && *cdnDomain != "" {
    contentService, err = web.NewS3ContentService(*s3Bucket, *cdnDomain)
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## 🐳 Phase 5: Containerization

### 5.1 Create Dockerfile for Backend

Create `Dockerfile` in root:

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ffmpeg

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/web

# Run stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates ffmpeg

WORKDIR /root/

COPY --from=builder /server .

EXPOSE 8080

CMD ["./server"]
```

### 5.2 Build and Push to ECR

```bash
# Create ECR repository
aws ecr create-repository --repository-name ${PROJECT_NAME}-backend

# Get ECR login
aws ecr get-login-password --region ${AWS_REGION} | \
  docker login --username AWS --password-stdin \
  <account-id>.dkr.ecr.${AWS_REGION}.amazonaws.com

# Build and push
docker build -t ${PROJECT_NAME}-backend .
docker tag ${PROJECT_NAME}-backend:latest \
  <account-id>.dkr.ecr.${AWS_REGION}.amazonaws.com/${PROJECT_NAME}-backend:latest
docker push <account-id>.dkr.ecr.${AWS_REGION}.amazonaws.com/${PROJECT_NAME}-backend:latest
```

---

## ☁️ Phase 6: ECS/Fargate Deployment

### 6.1 Create ECS Cluster

```bash
aws ecs create-cluster --cluster-name ${PROJECT_NAME}-cluster
```

### 6.2 Create Task Definition

Create `task-definition.json`:

```json
{
  "family": "tritontube-backend",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "arn:aws:iam::<account-id>:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::<account-id>:role/TritonTubeTaskRole",
  "containerDefinitions": [
    {
      "name": "tritontube-backend",
      "image": "<account-id>.dkr.ecr.us-east-1.amazonaws.com/tritontube-backend:latest",
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "DB_CONNECTION_STRING",
          "value": "postgres://admin:password@<rds-endpoint>:5432/tritontube"
        },
        {
          "name": "S3_BUCKET",
          "value": "tritontube-video-content"
        },
        {
          "name": "CDN_DOMAIN",
          "value": "d111111abcdef8.cloudfront.net"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/tritontube",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "backend"
        }
      }
    }
  ]
}
```

Register the task:
```bash
aws ecs register-task-definition --cli-input-json file://task-definition.json
```

### 6.3 Create Application Load Balancer

```bash
# Create ALB
aws elbv2 create-load-balancer \
  --name ${PROJECT_NAME}-alb \
  --subnets subnet-xxxxx subnet-yyyyy \
  --security-groups sg-xxxxx

# Create target group
aws elbv2 create-target-group \
  --name ${PROJECT_NAME}-tg \
  --protocol HTTP \
  --port 8080 \
  --vpc-id vpc-xxxxx \
  --target-type ip \
  --health-check-path /api/videos

# Create listener
aws elbv2 create-listener \
  --load-balancer-arn <alb-arn> \
  --protocol HTTP \
  --port 80 \
  --default-actions Type=forward,TargetGroupArn=<target-group-arn>
```

### 6.4 Create ECS Service with Auto Scaling

```bash
aws ecs create-service \
  --cluster ${PROJECT_NAME}-cluster \
  --service-name ${PROJECT_NAME}-service \
  --task-definition tritontube-backend \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-xxxxx,subnet-yyyyy],securityGroups=[sg-xxxxx],assignPublicIp=ENABLED}" \
  --load-balancers targetGroupArn=<target-group-arn>,containerName=tritontube-backend,containerPort=8080

# Set up auto scaling
aws application-autoscaling register-scalable-target \
  --service-namespace ecs \
  --scalable-dimension ecs:service:DesiredCount \
  --resource-id service/${PROJECT_NAME}-cluster/${PROJECT_NAME}-service \
  --min-capacity 2 \
  --max-capacity 10

# Create scaling policy
aws application-autoscaling put-scaling-policy \
  --policy-name cpu-scaling \
  --service-namespace ecs \
  --scalable-dimension ecs:service:DesiredCount \
  --resource-id service/${PROJECT_NAME}-cluster/${PROJECT_NAME}-service \
  --policy-type TargetTrackingScaling \
  --target-tracking-scaling-policy-configuration file://scaling-policy.json
```

Create `scaling-policy.json`:
```json
{
  "TargetValue": 70.0,
  "PredefinedMetricSpecification": {
    "PredefinedMetricType": "ECSServiceAverageCPUUtilization"
  },
  "ScaleOutCooldown": 60,
  "ScaleInCooldown": 60
}
```

---

## ⚡ Phase 7: Lambda for Video Processing (Optional)

### 7.1 Create Lambda Function for FFmpeg Processing

Create `lambda/video-processor/index.js`:

```javascript
const AWS = require('aws-sdk');
const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const s3 = new AWS.S3();

exports.handler = async (event) => {
  const bucket = event.Records[0].s3.bucket.name;
  const key = decodeURIComponent(event.Records[0].s3.object.key.replace(/\+/g, ' '));
  
  const downloadPath = `/tmp/${path.basename(key)}`;
  const outputDir = '/tmp/output';
  
  // Download video from S3
  const params = { Bucket: bucket, Key: key };
  const data = await s3.getObject(params).promise();
  fs.writeFileSync(downloadPath, data.Body);
  
  // Create output directory
  fs.mkdirSync(outputDir, { recursive: true });
  
  // Run FFmpeg to create DASH segments
  execSync(`ffmpeg -i ${downloadPath} -c:v libx264 -c:a aac -f dash ${outputDir}/manifest.mpd`);
  
  // Upload segments to S3
  const files = fs.readdirSync(outputDir);
  for (const file of files) {
    const filePath = path.join(outputDir, file);
    const fileContent = fs.readFileSync(filePath);
    const videoID = path.basename(key, path.extname(key));
    
    await s3.putObject({
      Bucket: 'tritontube-video-content',
      Key: `${videoID}/${file}`,
      Body: fileContent
    }).promise();
  }
  
  return { statusCode: 200, body: 'Processing complete' };
};
```

### 7.2 Deploy Lambda with FFmpeg Layer

```bash
# Create Lambda function
aws lambda create-function \
  --function-name ${PROJECT_NAME}-video-processor \
  --runtime nodejs18.x \
  --role arn:aws:iam::<account-id>:role/lambda-execution-role \
  --handler index.handler \
  --zip-file fileb://function.zip \
  --timeout 900 \
  --memory-size 3008 \
  --layers arn:aws:lambda:us-east-1:123456789012:layer:ffmpeg:1

# Add S3 trigger
aws lambda add-permission \
  --function-name ${PROJECT_NAME}-video-processor \
  --statement-id s3-trigger \
  --action lambda:InvokeFunction \
  --principal s3.amazonaws.com \
  --source-arn arn:aws:s3:::${PROJECT_NAME}-uploads

# Configure S3 event notification
aws s3api put-bucket-notification-configuration \
  --bucket ${PROJECT_NAME}-uploads \
  --notification-configuration file://s3-notification.json
```

---

## 🎨 Phase 8: Frontend Deployment

### 8.1 Update Frontend API Configuration

Update `src/config/api.ts`:

```typescript
const API_BASE_URL = process.env.REACT_APP_API_URL || 'https://<alb-dns-name>';
const CDN_BASE_URL = process.env.REACT_APP_CDN_URL || 'https://<cloudfront-domain>';

export const API_ENDPOINTS = {
  videos: `${API_BASE_URL}/api/videos`,
  upload: `${API_BASE_URL}/api/upload`,
  delete: (id: string) => `${API_BASE_URL}/api/delete/${id}`,
  thumbnail: (id: string) => `${CDN_BASE_URL}/${id}/thumbnail.jpg`,
  manifest: (id: string) => `${CDN_BASE_URL}/${id}/manifest.mpd`,
};
```

### 8.2 Build and Deploy Frontend

```bash
# Build React app
npm run build

# Sync to S3
aws s3 sync build/ s3://${PROJECT_NAME}-frontend/ --delete

# Invalidate CloudFront cache
aws cloudfront create-invalidation \
  --distribution-id <distribution-id> \
  --paths "/*"
```

---

## 🔐 Phase 9: IAM Roles and Permissions

### 9.1 Create ECS Task Role

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::tritontube-*/*",
        "arn:aws:s3:::tritontube-*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "sqs:SendMessage",
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage"
      ],
      "Resource": "arn:aws:sqs:*:*:tritontube-*"
    }
  ]
}
```

---

## 📊 Phase 10: Monitoring and Logging

### 10.1 Create CloudWatch Log Group

```bash
aws logs create-log-group --log-group-name /ecs/tritontube
```

### 10.2 Set Up CloudWatch Alarms

```bash
# High CPU alarm
aws cloudwatch put-metric-alarm \
  --alarm-name ${PROJECT_NAME}-high-cpu \
  --alarm-description "Alert when CPU exceeds 80%" \
  --metric-name CPUUtilization \
  --namespace AWS/ECS \
  --statistic Average \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=ServiceName,Value=${PROJECT_NAME}-service Name=ClusterName,Value=${PROJECT_NAME}-cluster
```

---

## 🚀 Phase 11: Deploy Script

Create `deploy.sh`:

```bash
#!/bin/bash
set -e

echo "🚀 Deploying TritonTube to AWS..."

# Build backend
echo "📦 Building backend Docker image..."
docker build -t tritontube-backend .

# Push to ECR
echo "⬆️ Pushing to ECR..."
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <account-id>.dkr.ecr.us-east-1.amazonaws.com
docker tag tritontube-backend:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/tritontube-backend:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/tritontube-backend:latest

# Update ECS service
echo "🔄 Updating ECS service..."
aws ecs update-service --cluster tritontube-cluster --service tritontube-service --force-new-deployment

# Build frontend
echo "🎨 Building frontend..."
cd frontend
npm install
npm run build

# Deploy frontend
echo "⬆️ Deploying frontend to S3..."
aws s3 sync build/ s3://tritontube-frontend/ --delete

# Invalidate CloudFront
echo "♻️ Invalidating CloudFront cache..."
aws cloudfront create-invalidation --distribution-id <distribution-id> --paths "/*"

echo "✅ Deployment complete!"
```

---

## 💰 Cost Estimation (Monthly)

| Service | Configuration | Estimated Cost |
|---------|--------------|----------------|
| **S3 Storage** | 100GB video content | $2.30 |
| **S3 Requests** | 1M GET requests | $0.40 |
| **CloudFront** | 1TB data transfer | $85.00 |
| **RDS (PostgreSQL)** | db.t3.micro | $15.00 |
| **ECS Fargate** | 2 tasks (0.5 vCPU, 1GB) | $30.00 |
| **ALB** | Standard load balancer | $18.00 |
| **Lambda** | 10k invocations | $2.00 |
| **CloudWatch Logs** | 10GB logs | $5.00 |
| **Data Transfer** | Various | $10.00 |
| **Total** | | **~$167.70/month** |

*Costs scale with usage, especially CloudFront bandwidth*

---

## 🔧 Troubleshooting

### Issue: ECS Tasks Not Starting
- Check CloudWatch logs in `/ecs/tritontube`
- Verify security groups allow traffic
- Ensure ECR image is accessible

### Issue: Videos Not Playing
- Verify CloudFront distribution is deployed
- Check S3 bucket CORS configuration
- Ensure manifest.mpd is properly formatted

### Issue: Database Connection Errors
- Confirm RDS security group allows ECS access
- Verify connection string is correct
- Check RDS is in same VPC as ECS

---

## 📚 Next Steps

1. **Set up CI/CD** with GitHub Actions or AWS CodePipeline
2. **Add ElastiCache** for video metadata caching
3. **Implement video transcoding** with multiple bitrates
4. **Set up Route53** for custom domain
5. **Add WAF** for security protection
6. **Enable X-Ray** for distributed tracing

---

## 🎉 Success!

Your TritonTube platform should now be running on AWS with:
- ✅ Scalable video storage with S3
- ✅ Global CDN delivery with CloudFront
- ✅ Auto-scaling backend with ECS/Fargate
- ✅ Managed PostgreSQL database with RDS
- ✅ Static frontend hosting

Access your application at:
- **Frontend:** `https://<cloudfront-frontend-domain>`
- **API:** `https://<alb-dns-name>/api/videos`
