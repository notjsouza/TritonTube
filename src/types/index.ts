// Import and re-export all video types
import type {
  Video,
  VideoUpload,
  VideoFilters,
  VideoState,
  ApiError,
  PaginatedResponse
} from './video.types';

export type {
  Video,
  VideoUpload,
  VideoFilters,
  VideoState,
  ApiError,
  PaginatedResponse
};

export interface AppState {
  videos: VideoState;
}
