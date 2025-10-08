// Lab 7: Implement a web server

package web

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	s.mux.HandleFunc("/api/upload", s.handleAPIUpload)
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
		log.Printf("FFmpeg conversion failed: %v\nOutput: %s", err, string(output))
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

		data, err := os.ReadFile(filepath.Join(tempDir, f.Name()))
		if err != nil {
			log.Printf("Failed to read segment file %s: %v", f.Name(), err)
			http.Error(w, "failed to read segment file", http.StatusInternalServerError)
			return
		}

		err = s.contentService.Write(videoId, f.Name(), data)
		if err != nil {
			log.Printf("Failed to write segment file %s: %v", f.Name(), err)
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
	log.Println("Video ID:", videoId)

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
	log.Println("Video ID:", videoId, "Filename:", filename)

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

	log.Println("Thumbnail request for video ID:", videoId)

	data, err := s.contentService.Read(videoId, "thumbnail.jpg")
	if err != nil {
		log.Printf("Failed to read thumbnail for %s: %v", videoId, err)
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
		log.Printf("Failed to list videos: %v", err)
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
		log.Printf("Failed to read video metadata: %v", err)
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
		log.Printf("Failed to create temp directory: %v", err)
		s.sendJSONError(w, "failed to create temporary directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, header.Filename)
	outFile, err := os.Create(videoPath)
	if err != nil {
		log.Printf("Failed to create video file: %v", err)
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
		log.Printf("FFmpeg conversion failed: %v\nOutput: %s", err, string(output))
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
		log.Printf("Warning: Thumbnail generation failed: %v\nOutput: %s", err, string(thumbnailOutput))
		// Don't fail the upload if thumbnail generation fails
	} else {
		log.Printf("Thumbnail generated successfully for video: %s", videoId)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Printf("Failed to read temp directory: %v", err)
		s.sendJSONError(w, "failed to read converted files", http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(tempDir, f.Name()))
		if err != nil {
			log.Printf("Failed to read segment file %s: %v", f.Name(), err)
			s.sendJSONError(w, "failed to read segment file", http.StatusInternalServerError)
			return
		}

		err = s.contentService.Write(videoId, f.Name(), data)
		if err != nil {
			log.Printf("Failed to write segment file %s: %v", f.Name(), err)
			s.sendJSONError(w, "failed to write segment file", http.StatusInternalServerError)
			return
		}
	}

	err = s.metadataService.Create(videoId, time.Now())
	if err != nil {
		log.Printf("Failed to save metadata: %v", err)
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
		log.Printf("Failed to read video metadata: %v", err)
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
		log.Printf("Warning: Failed to delete video content from storage nodes: %v", err)
		// Continue anyway to delete metadata
	}

	// Delete from metadata database
	err = s.metadataService.Delete(videoId)
	if err != nil {
		log.Printf("Failed to delete video metadata: %v", err)
		s.sendJSONError(w, "failed to delete video metadata", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully deleted video: %s", videoId)

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
