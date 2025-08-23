// Lab 8: Implement a network video content service (client using consistent hashing)

package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
	"time"

	"tritontube/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NetworkVideoContentService implements VideoContentService using a network of nodes.
type NetworkVideoContentService struct {
	proto.UnimplementedVideoContentAdminServiceServer
	mu           sync.RWMutex
	clients      map[string]proto.VideoContentClient
	nodes        []string
	nodeHashes   []uint64
	nodeMap      map[uint64]string
	fileRegistry map[string][]string
}

// Uncomment the following line to ensure NetworkVideoContentService implements VideoContentService
var _ VideoContentService = (*NetworkVideoContentService)(nil)

func NewNetworkVideoContentService(nodeAddrs []string) (*NetworkVideoContentService, error) {
	clients := make(map[string]proto.VideoContentClient)
	nodeMap := make(map[uint64]string)
	nodeHashes := make([]uint64, 0, len(nodeAddrs))

	for _, addr := range nodeAddrs {
		conn, err := grpc.NewClient(
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallSendMsgSize(32*1024*1024),
				grpc.MaxCallRecvMsgSize(32*1024*1024),
			),
		)

		if err != nil {
			return nil, fmt.Errorf("failed to connect to node %s: %v", addr, err)
		}

		client := proto.NewVideoContentClient(conn)
		clients[addr] = client
		hash := hashStringToUint64(addr)
		nodeHashes = append(nodeHashes, hash)
		nodeMap[hash] = addr
	}

	sort.Slice(nodeHashes, func(i, j int) bool { return nodeHashes[i] < nodeHashes[j] })

	return &NetworkVideoContentService{
		clients:      clients,
		nodes:        nodeAddrs,
		nodeHashes:   nodeHashes,
		nodeMap:      nodeMap,
		fileRegistry: make(map[string][]string),
	}, nil
}

func (n *NetworkVideoContentService) Write(videoId, filename string, data []byte) error {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	nodeID := n.getNodeForKey(key)
	client := n.clients[nodeID]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.WriteFileRequest{
		VideoId:  videoId,
		Filename: filename,
		Data:     data,
	}

	_, err := client.WriteFile(ctx, req)
	if err != nil {
		return err
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	found := false
	for _, item := range n.fileRegistry[videoId] {
		if item == filename {
			found = true
			break
		}
	}
	if !found {
		n.fileRegistry[videoId] = append(n.fileRegistry[videoId], filename)
	}
	return nil
}

func (n *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	nodeID := n.getNodeForKey(key)
	client := n.clients[nodeID]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.ReadFileRequest{
		VideoId:  videoId,
		Filename: filename,
	}

	res, err := client.ReadFile(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (n *NetworkVideoContentService) getNodeForKey(key string) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	hash := hashStringToUint64(key)
	index := sort.Search(len(n.nodeHashes), func(i int) bool {
		return n.nodeHashes[i] >= hash
	})

	if index == len(n.nodeHashes) {
		index = 0
	}

	return n.nodeMap[n.nodeHashes[index]]
}

func (n *NetworkVideoContentService) getNodeAddRemove(key string) string {
	hash := hashStringToUint64(key)
	index := sort.Search(len(n.nodeHashes), func(i int) bool {
		return n.nodeHashes[i] >= hash
	})

	if index == len(n.nodeHashes) {
		index = 0
	}

	return n.nodeMap[n.nodeHashes[index]]
}

func (n *NetworkVideoContentService) AddNode(ctx context.Context, req *proto.AddNodeRequest) (*proto.AddNodeResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	node := req.NodeAddress
	conn, err := grpc.NewClient(node, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, fmt.Errorf("[AddNode] failed to connect to new node %s: %v", node, err)
	}

	n.clients[node] = proto.NewVideoContentClient(conn)
	hash := hashStringToUint64(node)
	n.nodeMap[hash] = node
	n.nodeHashes = append(n.nodeHashes, hash)
	sort.Slice(n.nodeHashes, func(i, j int) bool { return n.nodeHashes[i] < n.nodeHashes[j] })
	n.nodes = append(n.nodes, node)
	migrated := 0

	for videoId, filenames := range n.fileRegistry {
		for _, filename := range filenames {
			key := fmt.Sprintf("%s/%s", videoId, filename)
			newNode := n.getNodeAddRemove(key)
			oldNode := n.findNodeBeforeAdding(node, key)

			if newNode != oldNode {
				ctxRead, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
				data, err := n.clients[oldNode].ReadFile(ctxRead, &proto.ReadFileRequest{
					VideoId:  videoId,
					Filename: filename,
				})
				cancelRead()
				if err != nil {
					continue
				}

				ctxWrite, cancelWrite := context.WithTimeout(context.Background(), 5*time.Second)
				_, err = n.clients[newNode].WriteFile(ctxWrite, &proto.WriteFileRequest{
					VideoId:  videoId,
					Filename: filename,
					Data:     data.Data,
				})
				cancelWrite()

				if err != nil {
					continue
				}

				migrated++
			}
		}
	}

	return &proto.AddNodeResponse{MigratedFileCount: int32(migrated)}, nil
}

func (n *NetworkVideoContentService) findNodeBeforeAdding(addedNode string, key string) string {
	tempHashes := make([]uint64, 0, len(n.nodeHashes))
	tempMap := make(map[uint64]string)

	for h, node := range n.nodeMap {
		if node != addedNode {
			tempHashes = append(tempHashes, h)
			tempMap[h] = node
		}
	}

	sort.Slice(tempHashes, func(i, j int) bool { return tempHashes[i] < tempHashes[j] })
	hash := hashStringToUint64(key)
	index := sort.Search(len(tempHashes), func(i int) bool {
		return tempHashes[i] >= hash
	})

	if index == len(tempHashes) {
		index = 0
	}

	return tempMap[tempHashes[index]]
}

func (n *NetworkVideoContentService) RemoveNode(ctx context.Context, req *proto.RemoveNodeRequest) (*proto.RemoveNodeResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	node := req.NodeAddress
	removedHash := hashStringToUint64(node)

	oldClient, ok := n.clients[node]
	if !ok {
		return nil, fmt.Errorf("node %s not found", node)
	}

	newHashes := make([]uint64, 0, len(n.nodeHashes))
	for _, h := range n.nodeHashes {
		if h != removedHash {
			newHashes = append(newHashes, h)
		}
	}
	n.nodeHashes = newHashes

	newNodes := make([]string, 0, len(n.nodes))
	for _, nAddr := range n.nodes {
		if nAddr != node {
			newNodes = append(newNodes, nAddr)
		}
	}
	n.nodes = newNodes

	delete(n.clients, node)
	delete(n.nodeMap, removedHash)

	migrated := 0
	for videoId, filenames := range n.fileRegistry {
		for _, filename := range filenames {
			key := fmt.Sprintf("%s/%s", videoId, filename)
			oldNode := node
			newNode := n.getNodeAddRemove(key)
			if newNode == oldNode {
				continue
			}

			ctxRead, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
			data, err := oldClient.ReadFile(ctxRead, &proto.ReadFileRequest{
				VideoId:  videoId,
				Filename: filename,
			})
			cancelRead()
			if err != nil {
				continue
			}

			ctxWrite, cancelWrite := context.WithTimeout(context.Background(), 5*time.Second)
			_, err = n.clients[newNode].WriteFile(ctxWrite, &proto.WriteFileRequest{
				VideoId:  videoId,
				Filename: filename,
				Data:     data.Data,
			})

			cancelWrite()
			if err != nil {
				continue
			}

			migrated++
		}
	}

	return &proto.RemoveNodeResponse{MigratedFileCount: int32(migrated)}, nil
}

func (n *NetworkVideoContentService) ListNodes(ctx context.Context, req *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	nodes := make([]string, len(n.nodes))
	copy(nodes, n.nodes)
	sort.Strings(nodes)

	return &proto.ListNodesResponse{Nodes: nodes}, nil
}

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}
