package main

import (
	"flag"
	"fmt"
	"net"
	"strings"
	"tritontube/internal/proto"
	"tritontube/internal/web"

	"google.golang.org/grpc"
)

// printUsage prints the usage information for the application
func printUsage() {
	fmt.Println("Usage: ./program [OPTIONS] METADATA_TYPE METADATA_OPTIONS CONTENT_TYPE CONTENT_OPTIONS")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  METADATA_TYPE         Metadata service type (sqlite, etcd)")
	fmt.Println("  METADATA_OPTIONS      Options for metadata service (e.g., db path)")
	fmt.Println("  CONTENT_TYPE          Content service type (fs, nw)")
	fmt.Println("  CONTENT_OPTIONS       Options for content service (e.g., base dir, network addresses)")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Example: ./program sqlite db.db fs /path/to/videos")
}

func main() {
	// Define flags
	port := flag.Int("port", 8080, "Port number for the web server")
	host := flag.String("host", "localhost", "Host address for the web server")
	adminPort := flag.Int("admin-port", 8081, "Port number for the admin gRPC server (for managing storage nodes)")

	// Set custom usage message
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if the correct number of positional arguments is provided
	if len(flag.Args()) != 4 {
		fmt.Println("Error: Incorrect number of arguments")
		printUsage()
		return
	}

	// Parse positional arguments
	metadataServiceType := flag.Arg(0)
	metadataServiceOptions := flag.Arg(1)
	contentServiceType := flag.Arg(2)
	contentServiceOptions := flag.Arg(3)

	// Validate port number (already an int from flag, check if positive)
	if *port <= 0 {
		fmt.Println("Error: Invalid port number:", *port)
		printUsage()
		return
	}

	// Construct metadata service
	var metadataService web.VideoMetadataService
	fmt.Println("Creating metadata service of type", metadataServiceType, "with options", metadataServiceOptions)

	switch metadataServiceType {
	case "sqlite":
		var err error
		metadataService, err = web.NewSQLiteVideoMetadataService(metadataServiceOptions)
		if err != nil {
			fmt.Printf("Error creating SQLite metadata service: %v\n", err)
			return
		}
	default:
		fmt.Printf("Error: Unsupported metadata service type: %s\n", metadataServiceType)
		return
	}

	// Construct content service
	var contentService web.VideoContentService
	fmt.Println("Creating content service of type", contentServiceType, "with options", contentServiceOptions)

	switch contentServiceType {
	case "fs":
		contentService = web.NewFSVideoContentService(contentServiceOptions)
	case "nw":
		storageAddrs := strings.Split(contentServiceOptions, ",")

		if len(storageAddrs) < 1 {
			fmt.Println("Error: Network content service requires at least one storage address")
			return
		}

		svc, err := web.NewNetworkVideoContentService(storageAddrs)

		if err != nil {
			fmt.Printf("Error creating network content service: %v\n", err)
			return
		}

		contentService = svc

		// Start admin gRPC server for managing storage nodes (add/remove/list)
		adminAddr := fmt.Sprintf("%s:%d", *host, *adminPort)
		go func() {
			lis, err := net.Listen("tcp", adminAddr)

			if err != nil {
				fmt.Println("Error starting admin listener:", err)
				return
			}

			grpcServer := grpc.NewServer()
			proto.RegisterVideoContentAdminServiceServer(grpcServer, svc)
			fmt.Println("Admin gRPC server listening at", adminAddr)

			if err := grpcServer.Serve(lis); err != nil {
				fmt.Println("Error serving admin gRPC server:", err)
				return
			}
		}()
	default:
		fmt.Printf("Error: Unsupported content service type: %s\n", contentServiceType)
		return
	}

	// Start the server
	server := web.NewServer(metadataService, contentService)
	listenAddr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", listenAddr)

	if err != nil {
		fmt.Println("Error starting listener:", err)
		return

	}
	defer lis.Close()

	fmt.Println("Starting web server on", listenAddr)
	if err := server.Start(lis); err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
}
