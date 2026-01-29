package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL not set")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}

	sqsClient := sqs.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	for {
		// Receive messages
		out, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:            &queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("receive message error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, m := range out.Messages {
			var payload struct {
				VideoId  string `json:"videoId"`
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal([]byte(*m.Body), &payload); err != nil {
				log.Printf("invalid message body: %v", err)
				// delete message to avoid poison
				sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
					QueueUrl:      &queueURL,
					ReceiptHandle: m.ReceiptHandle,
				})
				continue
			}

			log.Printf("Processing job: %s / %s", payload.VideoId, payload.Filename)

			// process each message inside a closure so that defers (cleanup) run per-iteration
			func() {
				// create tmp dir
				tmp, err := os.MkdirTemp("", "proc-*")
				if err != nil {
					log.Printf("mkdir temp failed: %v", err)
					return
				}
				defer os.RemoveAll(tmp)

				// download from uploads/<videoId>/<filename>
				srcKey := filepath.Join("uploads", payload.VideoId, payload.Filename)
				bucketName := os.Getenv("S3_BUCKET_NAME")
				if bucketName == "" {
					bucketName = "tritontube-video-content"
				}
				getResp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
					Bucket: &bucketName,
					Key:    &srcKey,
				})
				if err != nil {
					log.Printf("s3 get object failed: %v", err)
					// do not delete the message so it can be retried
					return
				}
				localPath := filepath.Join(tmp, payload.Filename)
				outf, err := os.Create(localPath)
				if err != nil {
					log.Printf("create local file failed: %v", err)
					getResp.Body.Close()
					return
				}
				_, err = io.Copy(outf, getResp.Body)
				outf.Close()
				getResp.Body.Close()
				if err != nil {
					log.Printf("copy local failed: %v", err)
					return
				}

				// run ffmpeg
				manifestPath := filepath.Join(tmp, "manifest.mpd")
				cmd := exec.Command("ffmpeg",
					"-i", localPath,
					"-c:v", "libx264",
					"-c:a", "aac",
					"-bf", "1",
					"-keyint_min", "120",
					"-g", "120",
					"-sc_threshold", "0",
					"-b:v", "3000k",
					"-b:a", "128k",
					"-f", "dash",
					"-use_timeline", "1",
					"-use_template", "1",
					"-init_seg_name", "init-$RepresentationID$.m4s",
					"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
					"-seg_duration", "4",
					manifestPath,
				)
				cmd.Dir = tmp
				outb, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("ffmpeg failed: %v, out: %s", err, string(outb))
					return
				}

				// Upload produced files in tmp directory to final content bucket under <videoId>/
				uploadBucket := os.Getenv("S3_BUCKET_NAME")
				if uploadBucket == "" {
					uploadBucket = "tritontube-video-content"
				}

				files, err := os.ReadDir(tmp)
				if err != nil {
					log.Printf("failed to list produced files: %v", err)
					return
				}

				for _, ff := range files {
					if ff.IsDir() {
						continue
					}
					name := ff.Name()
					data, err := os.ReadFile(filepath.Join(tmp, name))
					if err != nil {
						log.Printf("failed to read produced file %s: %v", name, err)
						continue
					}

					key := filepath.Join(payload.VideoId, name)
					contentType := "application/octet-stream"
					if name == "manifest.mpd" {
						contentType = "application/dash+xml"
					} else if strings.HasSuffix(name, ".m4s") {
						contentType = "video/iso.segment"
					} else if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, "thumbnail.jpg") {
						contentType = "image/jpeg"
					}

					_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
						Bucket:      aws.String(uploadBucket),
						Key:         aws.String(key),
						Body:        bytes.NewReader(data),
						ContentType: aws.String(contentType),
					})
					if err != nil {
						log.Printf("failed to upload produced file %s: %v", name, err)
						continue
					}
					log.Printf("Uploaded produced file to s3://%s/%s", uploadBucket, key)
				}

				log.Printf("Job completed for %s", payload.VideoId)

				// delete message after success
				_, _ = sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
					QueueUrl:      &queueURL,
					ReceiptHandle: m.ReceiptHandle,
				})
			}()
		}
	}
}
