## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         USERS / CLIENTS                         │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│              FRONTEND (React + TypeScript)                      │
│  - Single Page Application                                      │
│  - DASH.js video player                                         │
│  - Redux state management                                       │
└────────────────┬────────────────────────────────────────────────┘
                 │ HTTP/REST
                 ▼
┌────────────────────────────────────────────────────────────────┐
│         WEB SERVER (Go - Port 8080)                            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ HTTP REST API Layer                                      │  │
│  │  - GET /api/videos (list all)                            │  │
│  │  - GET /api/videos/:id (get details)                     │  │
│  │  - POST /api/upload (upload video)                       │  │
│  │  - DELETE /api/delete/:id (delete video)                 │  │
│  │  - GET /content/:id/:file (serve DASH segments)          │  │
│  │  - GET /thumbnail/:id (serve thumbnails)                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Video Processing Pipeline                                │  │
│  │  1. Accept MP4 upload                                    │  │
│  │  2. FFmpeg DASH conversion                               │  │
│  │  3. Generate thumbnail (first frame)                     │  │
│  │  4. Distribute segments to storage nodes                 │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────┬────────────────────────────────────────────┬─────────────┘
      │                                            │
      │ SQLite                                     │ gRPC
      ▼                                            ▼
┌──────────────────┐              ┌─────────────────────────────┐
│ METADATA SERVICE │              │ CONTENT SERVICE             │
│  (SQLite DB)     │              │ (Network/Distributed)       │
│                  │              │                             │
│ Stores:          │              │ Uses Consistent Hashing     │
│ - Video ID       │              └─────┬───────────────────────┘
│ - Title          │                    │
│ - Upload date    │                    │ Distributes to:
│ - Duration       │                    ▼
│ - Views          │      ┌─────────────────────────────────────┐
└──────────────────┘      │   STORAGE NODES (gRPC Servers)      │
                          │                                     │
                          │  ┌────────────────────────────────┐ │
                          │  │ Node 1 (Port 8090)             │ │
                          │  │ - Stores video segments        │ │
                          │  │ - Serves via gRPC              │ │
                          │  └────────────────────────────────┘ │
                          │  ┌────────────────────────────────┐ │
                          │  │ Node 2 (Port 8091)             │ │
                          │  │ - Stores video segments        │ │
                          │  │ - Serves via gRPC              │ │
                          │  └────────────────────────────────┘ │
                          │  ┌────────────────────────────────┐ │
                          │  │ Node 3 (Port 8092)             │ │
                          │  │ - Stores video segments        │ │
                          │  │ - Serves via gRPC              │ │
                          │  └────────────────────────────────┘ │
                          └─────────────────────────────────────┘
                                         ▲
                                         │ gRPC (Management)
                          ┌──────────────┴──────────────┐
                          │ ADMIN gRPC SERVER           │
                          │ (Port 8081)                 │
                          │                             │
                          │ - Add storage nodes         │
                          │ - Remove storage nodes      │
                          │ - List active nodes         │
                          └─────────────────────────────┘
```