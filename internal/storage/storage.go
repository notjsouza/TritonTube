// Lab 8: Implement a network video content service (server)

package storage

// Implement a network video content service (server)

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"tritontube/internal/proto"

	"google.golang.org/grpc"
)

type Server struct {
	proto.UnimplementedVideoContentServer
	BaseDir string
}

func (s *Server) WriteFile(ctx context.Context, req *proto.WriteFileRequest) (*proto.WriteFileResponse, error) {
	// Special command to delete entire video directory
	if req.Filename == ".DELETE_ALL" {
		dir := filepath.Join(s.BaseDir, req.VideoId)
		if err := os.RemoveAll(dir); err != nil {
			return &proto.WriteFileResponse{Success: false}, fmt.Errorf("failed to delete directory: %v", err)
		}
		return &proto.WriteFileResponse{Success: true}, nil
	}

	dir := filepath.Join(s.BaseDir, req.VideoId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &proto.WriteFileResponse{Success: false}, fmt.Errorf("failed to create directory: %v", err)
	}
	filePath := filepath.Join(dir, req.Filename)
	if err := os.WriteFile(filePath, req.Data, 0644); err != nil {
		return &proto.WriteFileResponse{Success: false}, fmt.Errorf("failed to write file: %v", err)
	}
	return &proto.WriteFileResponse{Success: true}, nil
}

func (s *Server) ReadFile(ctx context.Context, req *proto.ReadFileRequest) (*proto.ReadFileResponse, error) {
	filePath := filepath.Join(s.BaseDir, req.VideoId, req.Filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &proto.ReadFileResponse{}, fmt.Errorf("failed to read file: %v", err)
	}
	return &proto.ReadFileResponse{Data: data}, nil
}

func StartServer(host string, port int, baseDir string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(256*1024*1024),
		grpc.MaxSendMsgSize(256*1024*1024),
	)
	proto.RegisterVideoContentServer(s, &Server{BaseDir: baseDir})
	fmt.Printf("Starting server on %s:%d\n", host, port)
	return s.Serve(lis)
}
