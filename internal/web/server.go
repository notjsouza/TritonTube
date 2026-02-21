// Lab 7: Implement a web server

package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux
}

type indexPageVideo struct {
	Id         string
	EscapedId  string
	UploadTime string
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {
	return &server{
		metadataService: metadataService,
		contentService:  contentService,
	}
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()

	// API endpoints (JSON responses)
	s.mux.HandleFunc("/api/videos", s.handleAPIVideos)
	s.mux.HandleFunc("/api/videos/", s.handleAPIVideoDetail)
	s.mux.HandleFunc("/api/presign-upload", s.handleAPIPresignUpload)
	s.mux.HandleFunc("/api/upload", s.handleAPIUpload)
	s.mux.HandleFunc("/api/process", s.handleAPIProcess)
	s.mux.HandleFunc("/api/delete/", s.handleAPIDelete)

	// Content endpoints (binary responses)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/thumbnail/", s.handleThumbnail)

	// Legacy HTML endpoints
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/", s.handleIndex)

	// Wrap with CORS middleware
	handler := s.corsMiddleware(s.mux)
	return http.Serve(lis, handler)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metas, err := s.metadataService.List()
	if err != nil {
		http.Error(w, "failed to list videos", http.StatusInternalServerError)
		return
	}

	var pageData []indexPageVideo
	for _, m := range metas {
		pageData = append(pageData, indexPageVideo{
			Id:         m.Id,
			EscapedId:  url.PathEscape(m.Id),
			UploadTime: m.UploadedAt.Format(time.RFC1123),
		})
	}

	tmpl := template.Must(template.New("index").Parse(indexHTML))
	if err := tmpl.Execute(w, pageData); err != nil {
		http.Error(w, "template rendering failed", http.StatusInternalServerError)
	}
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100 MB limit for video uploads
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".mp4") {
		http.Error(w, "invalid file type", http.StatusBadRequest)
		return
	}

	videoId := strings.TrimSuffix(header.Filename, ".mp4")

	if meta, _ := s.metadataService.Read(videoId); meta != nil {
		http.Error(w, "video ID already exists", http.StatusBadRequest)
		return
	}

	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		http.Error(w, "failed to create temporary directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, header.Filename)
	outFile, err := os.Create(videoPath)
	if err != nil {
		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	io.Copy(outFile, file)

	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
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
	cmd.Dir = tempDir // Set working directory to temp directory

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("ffmpeg conversion failed", "video_id", videoId, "error", err, "output", string(output))
		http.Error(w, "video conversion failed", http.StatusInternalServerError)
		return
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		http.Error(w, "failed to read converted files", http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// Skip the original MP4 file - we only need DASH segments
		if strings.HasSuffix(f.Name(), ".mp4") {
			slog.Debug("skipping original mp4", "video_id", videoId, "file", f.Name())
			continue
		}

		data, err := os.ReadFile(filepath.Join(tempDir, f.Name()))
		if err != nil {
			slog.Error("failed to read segment file", "video_id", videoId, "file", f.Name(), "error", err)
			http.Error(w, "failed to read segment file", http.StatusInternalServerError)
			return
		}

		err = s.contentService.Write(videoId, f.Name(), data)
		if err != nil {
			slog.Error("failed to write segment file", "video_id", videoId, "file", f.Name(), "error", err)
			http.Error(w, "failed to write segment file", http.StatusInternalServerError)
			return
		}
	}

	err = s.metadataService.Create(videoId, time.Now())
	if err != nil {
		http.Error(w, "failed to save video metadata", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoId := r.URL.Path[len("/videos/"):]
	slog.Debug("video page request", "video_id", videoId)

	meta, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "Failed to read video metadata", http.StatusInternalServerError)
		return
	}

	if meta == nil {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Id         string
		UploadedAt string
	}{
		Id:         meta.Id,
		UploadedAt: meta.UploadedAt.Format(time.RFC1123),
	}

	tmpl := template.Must(template.New("video").Parse(videoHTML))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
	}
}

func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	// parse /content/<videoId>/<filename>
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid content path", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]
	slog.Debug("video content request", "video_id", videoId, "filename", filename)

	data, err := s.contentService.Read(videoId, filename)
	if err != nil {
		http.Error(w, "failed to read video content", http.StatusInternalServerError)
		return
	}

	// Set appropriate content type for DASH files
	if strings.HasSuffix(filename, ".mpd") {
		w.Header().Set("Content-Type", "application/dash+xml")
	} else if strings.HasSuffix(filename, ".m4s") {
		w.Header().Set("Content-Type", "video/mp4")
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	// parse /thumbnail/<videoId>
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoId := strings.TrimPrefix(r.URL.Path, "/thumbnail/")
	if videoId == "" {
		http.Error(w, "video ID required", http.StatusBadRequest)
		return
	}

	slog.Debug("thumbnail request", "video_id", videoId)

	data, err := s.contentService.Read(videoId, "thumbnail.jpg")
	if err != nil {
		slog.Warn("failed to read thumbnail", "video_id", videoId, "error", err)
		http.Error(w, "thumbnail not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// CORS middleware to allow frontend requests
func (s *server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from React dev server and production
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// API Response structures
type apiVideoResponse struct {
	Id           string `json:"id"`
	EscapedId    string `json:"escapedId"`
	UploadTime   string `json:"uploadTime"`
	UploadedAt   string `json:"uploadedAt"`
	ManifestUrl  string `json:"manifestUrl"`
	ThumbnailUrl string `json:"thumbnailUrl"`
	Status       string `json:"status"` // "processing", "ready", "error"
}

type apiVideosListResponse struct {
	Data    []apiVideoResponse `json:"data"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	Limit   int                `json:"limit"`
	HasMore bool               `json:"hasMore"`
}

type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// handleAPIVideos handles GET /api/videos - list all videos
func (s *server) handleAPIVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metas, err := s.metadataService.List()
	if err != nil {
		slog.Error("failed to list videos", "error", err)
		s.sendJSONError(w, "failed to list videos", http.StatusInternalServerError)
		return
	}

	videos := make([]apiVideoResponse, 0, len(metas))
	for _, m := range metas {
		videos = append(videos, apiVideoResponse{
			Id:           m.Id,
			EscapedId:    url.PathEscape(m.Id),
			UploadTime:   m.UploadedAt.Format(time.RFC3339),
			UploadedAt:   m.UploadedAt.Format(time.RFC3339),
			ManifestUrl:  "/content/" + url.PathEscape(m.Id) + "/manifest.mpd",
			ThumbnailUrl: "/thumbnail/" + url.PathEscape(m.Id),
			Status:       m.Status,
		})
	}

	response := apiVideosListResponse{
		Data:    videos,
		Total:   len(videos),
		Page:    1,
		Limit:   len(videos),
		HasMore: false,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleAPIVideoDetail handles GET /api/videos/{id} - get single video
func (s *server) handleAPIVideoDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoId := strings.TrimPrefix(r.URL.Path, "/api/videos/")
	if videoId == "" {
		s.sendJSONError(w, "video ID required", http.StatusBadRequest)
		return
	}

	meta, err := s.metadataService.Read(videoId)
	if err != nil {
		slog.Error("failed to read video metadata", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to read video metadata", http.StatusInternalServerError)
		return
	}

	if meta == nil {
		s.sendJSONError(w, "video not found", http.StatusNotFound)
		return
	}

	response := apiVideoResponse{
		Id:           meta.Id,
		EscapedId:    url.PathEscape(meta.Id),
		UploadTime:   meta.UploadedAt.Format(time.RFC3339),
		UploadedAt:   meta.UploadedAt.Format(time.RFC3339),
		ManifestUrl:  "/content/" + url.PathEscape(meta.Id) + "/manifest.mpd",
		ThumbnailUrl: "/thumbnail/" + url.PathEscape(meta.Id),
		Status:       meta.Status,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleAPIUpload handles POST /api/upload - upload a new video
func (s *server) handleAPIUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100 MB limit for video uploads
	if err != nil {
		s.sendJSONError(w, "invalid form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.sendJSONError(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".mp4") {
		s.sendJSONError(w, "invalid file type, only MP4 files are allowed", http.StatusBadRequest)
		return
	}

	videoId := strings.TrimSuffix(header.Filename, ".mp4")

	if meta, _ := s.metadataService.Read(videoId); meta != nil {
		s.sendJSONError(w, "video ID already exists", http.StatusBadRequest)
		return
	}

	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		slog.Error("failed to create temp directory", "error", err)
		s.sendJSONError(w, "failed to create temporary directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, header.Filename)
	outFile, err := os.Create(videoPath)
	if err != nil {
		slog.Error("failed to create video file", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	io.Copy(outFile, file)

	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
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
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("ffmpeg conversion failed", "video_id", videoId, "error", err, "output", string(output))
		s.sendJSONError(w, "video conversion failed", http.StatusInternalServerError)
		return
	}

	// Generate thumbnail from first frame
	thumbnailPath := filepath.Join(tempDir, "thumbnail.jpg")
	thumbnailCmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vframes", "1", // Extract only 1 frame
		"-ss", "00:00:00", // At 0 seconds (first frame)
		"-vf", "scale=320:-1", // Scale to 320px width, maintain aspect ratio
		"-q:v", "2", // High quality JPEG (1-31, lower is better)
		thumbnailPath,
	)
	thumbnailCmd.Dir = tempDir

	thumbnailOutput, err := thumbnailCmd.CombinedOutput()
	if err != nil {
		slog.Warn("thumbnail generation failed", "video_id", videoId, "error", err, "output", string(thumbnailOutput))
		// Don't fail the upload if thumbnail generation fails
	} else {
		slog.Info("thumbnail generated", "video_id", videoId)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		slog.Error("failed to read temp directory", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to read converted files", http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// Skip the original MP4 file - we only need DASH segments
		if strings.HasSuffix(f.Name(), ".mp4") {
			slog.Debug("skipping original mp4", "video_id", videoId, "file", f.Name())
			continue
		}

		data, err := os.ReadFile(filepath.Join(tempDir, f.Name()))
		if err != nil {
			slog.Error("failed to read segment file", "video_id", videoId, "file", f.Name(), "error", err)
			s.sendJSONError(w, "failed to read segment file", http.StatusInternalServerError)
			return
		}

		err = s.contentService.Write(videoId, f.Name(), data)
		if err != nil {
			slog.Error("failed to write segment file", "video_id", videoId, "file", f.Name(), "error", err)
			s.sendJSONError(w, "failed to write segment file", http.StatusInternalServerError)
			return
		}
	}

	err = s.metadataService.Create(videoId, time.Now())
	if err != nil {
		slog.Error("failed to save metadata", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to save video metadata", http.StatusInternalServerError)
		return
	}

	response := apiVideoResponse{
		Id:           videoId,
		EscapedId:    url.PathEscape(videoId),
		UploadTime:   time.Now().Format(time.RFC3339),
		UploadedAt:   time.Now().Format(time.RFC3339),
		ManifestUrl:  "/content/" + url.PathEscape(videoId) + "/manifest.mpd",
		ThumbnailUrl: "/thumbnail/" + url.PathEscape(videoId),
	}

	s.sendJSON(w, response, http.StatusCreated)
}

// handleAPIDelete handles DELETE /api/delete/{id} - delete a video
func (s *server) handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoId := strings.TrimPrefix(r.URL.Path, "/api/delete/")
	if videoId == "" {
		s.sendJSONError(w, "video ID required", http.StatusBadRequest)
		return
	}

	// Check if video exists
	meta, err := s.metadataService.Read(videoId)
	if err != nil {
		slog.Error("failed to read video metadata", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to read video metadata", http.StatusInternalServerError)
		return
	}

	if meta == nil {
		s.sendJSONError(w, "video not found", http.StatusNotFound)
		return
	}

	// Delete content files from storage nodes
	err = s.contentService.DeleteAll(videoId)
	if err != nil {
		slog.Warn("failed to delete video content from storage", "video_id", videoId, "error", err)
		// Continue anyway to delete metadata
	}

	// Delete original upload file from uploads bucket if using S3
	if s3svc, ok := s.contentService.(*S3VideoContentService); ok {
		uploadBucket := os.Getenv("S3_UPLOAD_BUCKET")
		if uploadBucket == "" {
			uploadBucket = "tritontube-uploads"
		}

		// Delete the entire folder in uploads bucket
		uploadPrefix := fmt.Sprintf("uploads/%s/", videoId)
		slog.Info("deleting upload files", "video_id", videoId, "bucket", uploadBucket, "prefix", uploadPrefix)

		listInput := &s3.ListObjectsV2Input{
			Bucket: aws.String(uploadBucket),
			Prefix: aws.String(uploadPrefix),
		}

		listOutput, err := s3svc.client.ListObjectsV2(context.TODO(), listInput)
		if err != nil {
			slog.Warn("failed to list upload files for deletion", "video_id", videoId, "prefix", uploadPrefix, "error", err)
		} else if len(listOutput.Contents) > 0 {
			for _, obj := range listOutput.Contents {
				_, err := s3svc.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
					Bucket: aws.String(uploadBucket),
					Key:    obj.Key,
				})
				if err != nil {
					slog.Warn("failed to delete upload file", "video_id", videoId, "key", *obj.Key, "error", err)
				} else {
					slog.Info("deleted upload file", "video_id", videoId, "key", *obj.Key)
				}
			}
		}
	}

	// Delete from metadata database
	err = s.metadataService.Delete(videoId)
	if err != nil {
		slog.Error("failed to delete video metadata", "video_id", videoId, "error", err)
		s.sendJSONError(w, "failed to delete video metadata", http.StatusInternalServerError)
		return
	}

	slog.Info("video deleted", "video_id", videoId)

	response := map[string]interface{}{
		"success": true,
		"message": "video deleted successfully (metadata and content)",
		"id":      videoId,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// Helper functions for JSON responses
func (s *server) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *server) sendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

// ---- New endpoints: presign-upload and process ----

// handleAPIPresignUpload returns a presigned PUT URL for direct S3 upload.
// Request: POST with JSON { "videoId": "my-video-id", "filename": "my.mp4" }
// Response: { "url": "https://...", "headers": {} }
func (s *server) handleAPIPresignUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		VideoId  string `json:"videoId"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.sendJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.VideoId == "" || body.Filename == "" {
		s.sendJSONError(w, "videoId and filename are required", http.StatusBadRequest)
		return
	}

	// Check if video already exists to prevent race conditions
	existingMeta, err := s.metadataService.Read(body.VideoId)
	if err != nil {
		slog.Warn("failed to check existing video metadata", "video_id", body.VideoId, "error", err)
		// Continue anyway - better to allow upload than block on metadata check failure
	} else if existingMeta != nil {
		slog.Info("upload rejected: video already exists", "video_id", body.VideoId, "status", existingMeta.Status)
		s.sendJSONError(w, fmt.Sprintf("video ID '%s' already exists - please delete the existing video first or use a different ID", body.VideoId), http.StatusConflict)
		return
	}

	// Build S3 key under uploads/ prefix to separate raw uploads
	key := filepath.Join("uploads", body.VideoId, body.Filename)

	// Use AWS SDK v2 to create a presigned PUT URL
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		slog.Error("failed to load AWS config for presign", "video_id", body.VideoId, "error", err)
		s.sendJSONError(w, "failed to create presigned url", http.StatusInternalServerError)
		return
	}
	s3client := s3.NewFromConfig(cfg)
	presigner := s3.NewPresignClient(s3client)

	// PutObject presign - use uploads bucket
	bucketName := GetS3UploadsBucketFromEnv()

	// Detect content type from filename extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(strings.ToLower(body.Filename), ".mp4") {
		contentType = "video/mp4"
	} else if strings.HasSuffix(strings.ToLower(body.Filename), ".webm") {
		contentType = "video/webm"
	}

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	presignResp, err := presigner.PresignPutObject(context.TODO(), putInput)
	if err != nil {
		slog.Error("failed to presign PUT object", "video_id", body.VideoId, "key", key, "error", err)
		s.sendJSONError(w, "failed to create presigned url", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"url":    presignResp.URL,
		"method": "PUT",
		"key":    key,
	}
	s.sendJSON(w, resp, http.StatusOK)
}

// handleAPIProcess accepts a notification from the client after a direct S3 upload
// and enqueues processing (here we'll spawn a goroutine as a simple worker for dev).
// Request: POST { "videoId": "...", "filename": "..." }
func (s *server) handleAPIProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		VideoId  string `json:"videoId"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.sendJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.VideoId == "" || body.Filename == "" {
		s.sendJSONError(w, "videoId and filename are required", http.StatusBadRequest)
		return
	}

	// Create metadata entry immediately with "processing" status
	if err := s.metadataService.CreateWithStatus(body.VideoId, time.Now(), "processing"); err != nil {
		slog.Error("failed to create metadata with processing status", "video_id", body.VideoId, "error", err)
		// Check if it already exists
		existingMeta, readErr := s.metadataService.Read(body.VideoId)
		if readErr == nil && existingMeta != nil {
			slog.Warn("video already exists", "video_id", body.VideoId, "status", existingMeta.Status, "uploaded_at", existingMeta.UploadedAt)
			s.sendJSONError(w, fmt.Sprintf("video '%s' already exists or is being processed", body.VideoId), http.StatusConflict)
			return
		}
		// If we can't read it either, there's a bigger problem - fail the request
		slog.Error("cannot create or read metadata", "video_id", body.VideoId, "create_error", err, "read_error", readErr)
		s.sendJSONError(w, "failed to initialize video processing - please try again or use a different video ID", http.StatusInternalServerError)
		return
	}

	// If SQS queue URL is configured, enqueue a message and return immediately
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL != "" {
		// Build message body
		msgBody, _ := json.Marshal(map[string]string{"videoId": body.VideoId, "filename": body.Filename})
		// Load AWS config with explicit region
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-west-1"
		}
		cfg, cfgErr := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
		if cfgErr == nil {
			sqsClient := sqs.NewFromConfig(cfg)
			_, sendErr := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
				QueueUrl:    aws.String(queueURL),
				MessageBody: aws.String(string(msgBody)),
			})
			if sendErr == nil {
				slog.Info("job enqueued", "video_id", body.VideoId, "filename", body.Filename)
				resp := map[string]interface{}{
					"status":  "enqueued",
					"videoId": body.VideoId,
				}
				s.sendJSON(w, resp, http.StatusAccepted)
				return
			}
			slog.Error("failed to send SQS message", "video_id", body.VideoId, "error", sendErr)
		} else {
			slog.Error("failed to load AWS config for SQS send", "error", cfgErr)
		}
		// If SQS send failed, fall back to in-process worker
	}

	// Launch background goroutine to download the uploaded file from uploads/ and run processing
	go func(videoId, filename string) {
		bgLog := slog.With("video_id", videoId, "filename", filename, "worker", "background")

		// Create temp dir for processing
		tmp, err := os.MkdirTemp("", "proc-*")
		if err != nil {
			bgLog.Error("failed to create temp dir", "error", err)
			return
		}
		defer os.RemoveAll(tmp)

		// Download the uploaded file from S3 (uploads/<videoId>/<filename>) into tmp
		srcKey := filepath.Join("uploads", videoId, filename)

		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			bgLog.Error("failed to load AWS config", "error", err)
			return
		}
		s3client := s3.NewFromConfig(cfg)

		bucketName := GetS3UploadsBucketFromEnv()
		getResp, err := s3client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(srcKey),
		})
		if err != nil {
			bgLog.Error("failed to download uploaded file", "key", srcKey, "error", err)
			return
		}
		defer getResp.Body.Close()

		localPath := filepath.Join(tmp, filename)
		out, err := os.Create(localPath)
		if err != nil {
			bgLog.Error("failed to create local file", "path", localPath, "error", err)
			return
		}
		_, err = io.Copy(out, getResp.Body)
		out.Close()
		if err != nil {
			bgLog.Error("failed to write local file", "error", err)
			return
		}

		// Run FFmpeg to produce DASH segments into tmp
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
			bgLog.Error("ffmpeg failed", "error", err, "output", string(outb))
			return
		}

		// Upload generated files to final video location using s.contentService.Write
		files, err := os.ReadDir(tmp)
		if err != nil {
			bgLog.Error("readdir failed", "error", err)
			return
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if strings.HasSuffix(f.Name(), ".mp4") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(tmp, f.Name()))
			if err != nil {
				bgLog.Error("read file failed", "file", f.Name(), "error", err)
				return
			}
			if err := s.contentService.Write(videoId, f.Name(), data); err != nil {
				bgLog.Error("write to content service failed", "file", f.Name(), "error", err)
				return
			}
		}

		// Update metadata status to ready
		if err := s.metadataService.UpdateStatus(videoId, "ready"); err != nil {
			bgLog.Error("failed to update metadata status", "error", err)
			// Still log completion even if status update fails
		}

		bgLog.Info("processing completed")
	}(body.VideoId, body.Filename)

	// Respond immediately
	resp := map[string]interface{}{
		"status":  "processing_started",
		"videoId": body.VideoId,
	}
	s.sendJSON(w, resp, http.StatusAccepted)
}
