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
		conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(2*time.Second))

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
	fmt.Printf("[Write] Routing file %s to node %s\n", key, nodeID)

	client := n.clients[nodeID]
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &proto.WriteFileRequest{
		VideoId:  videoId,
		Filename: filename,
		Data:     data,
	}

	_, err := client.WriteFile(ctx, req)
	if err != nil {
		fmt.Printf("[Write] Error writing file %s to node %s: %v\n", key, nodeID, err)
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
		fmt.Printf("[Write] Registered file %s for video %s\n", filename, videoId)
	}
	return nil
}

func (n *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	nodeID := n.getNodeForKey(key)
	fmt.Printf("[Read] Routing file %s to node %s\n", key, nodeID)

	client := n.clients[nodeID]
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &proto.ReadFileRequest{
		VideoId:  videoId,
		Filename: filename,
	}

	res, err := client.ReadFile(ctx, req)
	if err != nil {
		fmt.Printf("[Read] Error reading file %s from node %s: %v\n", key, nodeID, err)
		return nil, err
	}

	fmt.Printf("[Read] Successfully read file %s from node %s (%d bytes)\n", key, nodeID, len(res.Data))
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

func (n *NetworkVideoContentService) AddNode(ctx context.Context, req *proto.AddNodeRequest) (*proto.AddNodeResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	node := req.NodeAddress
	fmt.Printf("[AddNode] Adding node: %s\n", node)

	// Connect to the new node
	conn, err := grpc.Dial(node, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("[AddNode] failed to connect to new node %s: %v", node, err)
	}
	n.clients[node] = proto.NewVideoContentClient(conn)

	// Update consistent hash ring
	hash := hashStringToUint64(node)
	n.nodeMap[hash] = node
	n.nodeHashes = append(n.nodeHashes, hash)
	sort.Slice(n.nodeHashes, func(i, j int) bool { return n.nodeHashes[i] < n.nodeHashes[j] })
	n.nodes = append(n.nodes, node)

	migrated := 0

	// Reassign any file whose new node is different
	for videoId, filenames := range n.fileRegistry {
		for _, filename := range filenames {
			key := fmt.Sprintf("%s/%s", videoId, filename)
			newNode := n.getNodeForKey(key)
			oldNode := n.findNodeBeforeAdding(node, key)

			fmt.Printf("[AddNode] Checking file %s: oldNode=%s, newNode=%s\n", key, oldNode, newNode)

			if newNode != oldNode {
				fmt.Printf("[AddNode] Migrating %s from %s to %s\n", key, oldNode, newNode)

				ctxRead, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
				data, err := n.clients[oldNode].ReadFile(ctxRead, &proto.ReadFileRequest{
					VideoId:  videoId,
					Filename: filename,
				})
				cancelRead()
				if err != nil {
					fmt.Printf("[AddNode] Failed to read %s from old node %s: %v\n", key, oldNode, err)
					continue
				}

				ctxWrite, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
				_, err = n.clients[newNode].WriteFile(ctxWrite, &proto.WriteFileRequest{
					VideoId:  videoId,
					Filename: filename,
					Data:     data.Data,
				})
				cancelWrite()
				if err != nil {
					fmt.Printf("[AddNode] Failed to write %s to new node %s: %v\n", key, newNode, err)
					continue
				}

				migrated++
			}
		}
	}

	fmt.Printf("[AddNode] Migration complete. %d files moved to node %s\n", migrated, node)
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
	fmt.Printf("[RemoveNode] Removing node: %s\n", node)
	removedHash := hashStringToUint64(node)

	delete(n.clients, node)
	delete(n.nodeMap, removedHash)

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

	migrated := 0

	// Migrate files that were stored on the removed node
	for videoId, filenames := range n.fileRegistry {
		for _, filename := range filenames {
			key := fmt.Sprintf("%s/%s", videoId, filename)
			oldNode := node
			newNode := n.getNodeForKey(key)

			if newNode == oldNode {
				fmt.Printf("[RemoveNode] WARNING: file %s would be mapped back to removed node %s, skipping\n", key, oldNode)
				continue
			}

			fmt.Printf("[RemoveNode] Migrating %s from %s to %s\n", key, oldNode, newNode)

			ctxRead, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
			data, err := n.clients[oldNode].ReadFile(ctxRead, &proto.ReadFileRequest{
				VideoId:  videoId,
				Filename: filename,
			})
			cancelRead()
			if err != nil {
				fmt.Printf("[RemoveNode] Failed to read %s from node %s: %v\n", key, oldNode, err)
				continue
			}

			ctxWrite, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
			_, err = n.clients[newNode].WriteFile(ctxWrite, &proto.WriteFileRequest{
				VideoId:  videoId,
				Filename: filename,
				Data:     data.Data,
			})
			cancelWrite()
			if err != nil {
				fmt.Printf("[RemoveNode] Failed to write %s to node %s: %v\n", key, newNode, err)
				continue
			}

			migrated++
		}
	}

	fmt.Printf("[RemoveNode] Migration complete. %d files moved away from node %s\n", migrated, node)
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
