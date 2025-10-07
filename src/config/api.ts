// API Configuration
export const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

export const API_ENDPOINTS = {
  VIDEOS: `${API_BASE_URL}/api/videos`,
  UPLOAD: `${API_BASE_URL}/api/upload`,
  VIDEO_DETAIL: (id: string) => `${API_BASE_URL}/api/videos/${id}`,
  VIDEO_DELETE: (id: string) => `${API_BASE_URL}/api/videos/${id}`,
  VIDEO_CONTENT: (id: string) => `${API_BASE_URL}/content/${id}/manifest.mpd`,
} as const;

export const APP_CONFIG = {
  MAX_FILE_SIZE: 100 * 1024 * 1024, // 100MB
  ALLOWED_FILE_TYPES: ['video/mp4'],
  VIDEOS_PER_PAGE: 12,
} as const;
