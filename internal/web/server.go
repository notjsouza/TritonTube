// Lab 7: Implement a web server

package web

import (
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
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/", s.handleIndex)

	return http.Serve(lis, s.mux)
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

	err := r.ParseMultipartForm(10 << 20)
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

	_, err = cmd.CombinedOutput()
	if err != nil {
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
			http.Error(w, "failed to read segment file", http.StatusInternalServerError)
			return
		}

		err = s.contentService.Write(videoId, f.Name(), data)
		if err != nil {
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

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
