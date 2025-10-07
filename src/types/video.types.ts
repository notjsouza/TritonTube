export interface Video {
  id: string;
  escapedId: string;
  uploadTime: string;
  uploadedAt: string;
  title?: string;
  description?: string;
  duration?: number;
  fileSize?: number;
  thumbnailUrl?: string;
  manifestUrl?: string;
}

export interface VideoUpload {
  id: string;
  file: File;
  progress: number;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  error?: string;
}

export interface ApiError {
  message: string;
  status: number;
  code?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
}

export interface VideoFilters {
  search?: string;
  sortBy?: 'uploadTime' | 'title' | 'duration';
  sortOrder?: 'asc' | 'desc';
}

export interface VideoState {
  videos: Video[];
  loading: boolean;
  error: string | null;
  currentVideo: Video | null;
  uploads: VideoUpload[];
  filters: VideoFilters;
  pagination: {
    page: number;
    limit: number;
    total: number;
    hasMore: boolean;
  };
}
