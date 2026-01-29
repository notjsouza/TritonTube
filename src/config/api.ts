// API Configuration
export const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://tritontube-alb-371777292.us-west-1.elb.amazonaws.com';
export const VIDEO_CDN_URL = process.env.REACT_APP_VIDEO_CDN || 'https://d2ujeim8ok9614.cloudfront.net';

export const API_ENDPOINTS = {
  VIDEOS: `${API_BASE_URL}/api/videos`,
  UPLOAD: `${API_BASE_URL}/api/upload`,
  VIDEO_DETAIL: (id: string) => `${API_BASE_URL}/api/videos/${id}`,
  VIDEO_DELETE: (id: string) => `${API_BASE_URL}/api/videos/${id}`,
  VIDEO_CONTENT: (id: string) => `${VIDEO_CDN_URL}/${id}/manifest.mpd`,
  THUMBNAIL: (id: string) => `${VIDEO_CDN_URL}/${id}/thumbnail.jpg`,
} as const;

export const APP_CONFIG = {
  MAX_FILE_SIZE: 100 * 1024 * 1024, // 100MB
  ALLOWED_FILE_TYPES: ['video/mp4'],
  VIDEOS_PER_PAGE: 12,
} as const;
