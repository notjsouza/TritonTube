import { Video, PaginatedResponse } from '../types';

// Mock data for development
const mockVideos: Video[] = [
  {
    id: 'video1',
    escapedId: 'video1',
    uploadTime: '2024-01-15T10:30:00Z',
    uploadedAt: '2024-01-15T10:30:00Z',
    title: 'Sample Video 1',
    description: 'This is a sample video for testing the TritonTube interface',
    duration: 120,
    fileSize: 15 * 1024 * 1024, // 15MB
    thumbnailUrl: '',
    manifestUrl: '/content/video1/manifest.mpd',
  },
  {
    id: 'video2',
    escapedId: 'video2',
    uploadTime: '2024-01-14T15:45:00Z',
    uploadedAt: '2024-01-14T15:45:00Z',
    title: 'Demo Video 2',
    description: 'Another sample video demonstrating the video grid layout',
    duration: 85,
    fileSize: 12 * 1024 * 1024, // 12MB
    thumbnailUrl: '',
    manifestUrl: '/content/video2/manifest.mpd',
  },
  {
    id: 'video3',
    escapedId: 'video3',
    uploadTime: '2024-01-13T09:20:00Z',
    uploadedAt: '2024-01-13T09:20:00Z',
    title: 'Test Video 3',
    description: 'A third video to show multiple items in the grid',
    duration: 200,
    fileSize: 25 * 1024 * 1024, // 25MB
    thumbnailUrl: '',
    manifestUrl: '/content/video3/manifest.mpd',
  },
];

// Simulate API delay
const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export const mockApi = {
  async fetchVideos(params: {
    page?: number;
    limit?: number;
    search?: string;
    sortBy?: string;
    sortOrder?: string;
  } = {}): Promise<PaginatedResponse<Video>> {
    await delay(500); // Simulate network delay

    const { page = 1, limit = 12, search = '', sortBy = 'uploadTime', sortOrder = 'desc' } = params;

    let filteredVideos = [...mockVideos];

    // Apply search filter
    if (search) {
      filteredVideos = filteredVideos.filter(video =>
        video.title?.toLowerCase().includes(search.toLowerCase()) ||
        video.description?.toLowerCase().includes(search.toLowerCase()) ||
        video.id.toLowerCase().includes(search.toLowerCase())
      );
    }

    // Apply sorting
    filteredVideos.sort((a, b) => {
      let aValue: any;
      let bValue: any;

      switch (sortBy) {
        case 'title':
          aValue = a.title || a.id;
          bValue = b.title || b.id;
          break;
        case 'duration':
          aValue = a.duration || 0;
          bValue = b.duration || 0;
          break;
        case 'uploadTime':
        default:
          aValue = new Date(a.uploadTime || a.uploadedAt);
          bValue = new Date(b.uploadTime || b.uploadedAt);
          break;
      }

      if (sortOrder === 'asc') {
        return aValue < bValue ? -1 : aValue > bValue ? 1 : 0;
      } else {
        return aValue > bValue ? -1 : aValue < bValue ? 1 : 0;
      }
    });

    // Apply pagination
    const startIndex = (page - 1) * limit;
    const endIndex = startIndex + limit;
    const paginatedVideos = filteredVideos.slice(startIndex, endIndex);

    return {
      data: paginatedVideos,
      total: filteredVideos.length,
      page,
      limit,
      hasMore: endIndex < filteredVideos.length,
    };
  },

  async uploadVideo(file: File): Promise<Video> {
    await delay(2000); // Simulate upload time

    // Create a new video entry
    const newVideo: Video = {
      id: `video_${Date.now()}`,
      escapedId: `video_${Date.now()}`,
      uploadTime: new Date().toISOString(),
      uploadedAt: new Date().toISOString(),
      title: file.name.replace('.mp4', ''),
      description: `Uploaded video: ${file.name}`,
      duration: Math.floor(Math.random() * 300) + 60, // Random duration between 1-6 minutes
      fileSize: file.size,
      thumbnailUrl: '',
      manifestUrl: `/content/video_${Date.now()}/manifest.mpd`,
    };

    // Add to mock videos list
    mockVideos.unshift(newVideo);

    return newVideo;
  },

  async deleteVideo(videoId: string): Promise<void> {
    await delay(300);
    
    const index = mockVideos.findIndex(video => video.id === videoId);
    if (index > -1) {
      mockVideos.splice(index, 1);
    }
  },

  async getVideo(videoId: string): Promise<Video | null> {
    await delay(200);
    
    return mockVideos.find(video => video.id === videoId || video.escapedId === videoId) || null;
  },
};
