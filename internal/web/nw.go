// Lab 8: Implement a network video content service (client using consistent hashing)

package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	"tritontube/internal/proto"

	"google.golang.org/grpc"
)

// NetworkVideoContentService implements VideoContentService using a network of nodes.
type NetworkVideoContentService struct {
	nodes   map[string]proto.VideoContentClient
	ring    []uint64
	ringMap map[uint64]string
}

// Uncomment the following line to ensure NetworkVideoContentService implements VideoContentService
var _ VideoContentService = (*NetworkVideoContentService)(nil)

func NewNetworkVideoContentService(addresses []string) (*NetworkVideoContentService, error) {
	clients := make(map[string]proto.VideoContentClient)
	ringMap := make(map[uint64]string)
	var ring []uint64
	for _, addr := range addresses {
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %v", addr, err)
		}
		clients[addr] = proto.NewVideoContentClient(conn)
		hash := hashStringToUint64(addr)
		ring = append(ring, hash)
		ringMap[hash] = addr
	}
	sort.Slice(ring, func(i, j int) bool { return ring[i] < ring[j] })
	return &NetworkVideoContentService{
		nodes:   clients,
		ring:    ring,
		ringMap: ringMap,
	}, nil
}

func (n *NetworkVideoContentService) getNode(key string) proto.VideoContentClient {
	hash := hashStringToUint64(key)
	for _, nodeHash := range n.ring {
		if hash <= nodeHash {
			return n.nodes[n.ringMap[nodeHash]]
		}
	}
	return n.nodes[n.ringMap[n.ring[0]]]
}

func (n *NetworkVideoContentService) Write(videoId, filename string, data []byte) error {
	client := n.getNode(fmt.Sprintf("%s:%s", videoId, filename))
	_, err := client.WriteFile(context.Background(), &proto.WriteFileRequest{
		VideoId:  videoId,
		Filename: filename,
		Data:     data,
	})
	return err
}

func (n *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	client := n.getNode(fmt.Sprintf("%s:%s", videoId, filename))
	res, err := client.ReadFile(context.Background(), &proto.ReadFileRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}
