import { Video, PaginatedResponse } from '../types';
import { API_BASE_URL } from '../config/api';

class ApiError extends Error {
  constructor(public status: number, message: string, public code?: string) {
    super(message);
    this.name = 'ApiError';
  }
}

// Helper function to handle API responses
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
    try {
      const errorData = await response.json();
      errorMessage = errorData.message || errorData.error || errorMessage;
    } catch {
      // If error response is not JSON, use status text
    }
    throw new ApiError(response.status, errorMessage);
  }

  // For 204 No Content, return empty object
  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}

export const api = {
  /**
   * Fetch all videos with optional filters
   */
  async fetchVideos(params: {
    page?: number;
    limit?: number;
    search?: string;
    sortBy?: string;
    sortOrder?: string;
  } = {}): Promise<PaginatedResponse<Video>> {
    const queryParams = new URLSearchParams();
    
    if (params.page) queryParams.append('page', params.page.toString());
    if (params.limit) queryParams.append('limit', params.limit.toString());
    if (params.search) queryParams.append('search', params.search);
    if (params.sortBy) queryParams.append('sortBy', params.sortBy);
    if (params.sortOrder) queryParams.append('sortOrder', params.sortOrder);

    const url = `${API_BASE_URL}/api/videos${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
    
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    return handleResponse<PaginatedResponse<Video>>(response);
  },

  /**
   * Fetch a single video by ID
   */
  async getVideo(videoId: string): Promise<Video> {
    const response = await fetch(`${API_BASE_URL}/api/videos/${encodeURIComponent(videoId)}`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    return handleResponse<Video>(response);
  },

  /**
   * Upload a new video file
   */
  async uploadVideo(file: File, onProgress?: (progress: number) => void): Promise<Video> {
    return new Promise((resolve, reject) => {
      const formData = new FormData();
      formData.append('file', file);

      const xhr = new XMLHttpRequest();

      // Track upload progress
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable && onProgress) {
          const percentComplete = (event.loaded / event.total) * 100;
          onProgress(percentComplete);
        }
      });

      // Handle completion
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            const response = JSON.parse(xhr.responseText);
            resolve(response);
          } catch (error) {
            reject(new ApiError(xhr.status, 'Invalid JSON response'));
          }
        } else {
          let errorMessage = `HTTP ${xhr.status}: ${xhr.statusText}`;
          try {
            const errorData = JSON.parse(xhr.responseText);
            errorMessage = errorData.message || errorData.error || errorMessage;
          } catch {
            // Use default error message
          }
          reject(new ApiError(xhr.status, errorMessage));
        }
      });

      // Handle errors
      xhr.addEventListener('error', () => {
        reject(new ApiError(0, 'Network error occurred'));
      });

      xhr.addEventListener('abort', () => {
        reject(new ApiError(0, 'Upload aborted'));
      });

      // Send the request
      xhr.open('POST', `${API_BASE_URL}/api/upload`);
      xhr.send(formData);
    });
  },

  /**
   * Delete a video by ID
   */
  async deleteVideo(videoId: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/api/delete/${encodeURIComponent(videoId)}`, {
      method: 'DELETE',
      headers: {
        'Accept': 'application/json',
      },
    });

    await handleResponse<void>(response);
  },

  /**
   * Get the manifest URL for a video
   */
  getManifestUrl(videoId: string): string {
    return `${API_BASE_URL}/content/${encodeURIComponent(videoId)}/manifest.mpd`;
  },

  /**
   * Health check endpoint
   */
  async healthCheck(): Promise<boolean> {
    try {
      const response = await fetch(`${API_BASE_URL}/api/videos`, {
        method: 'HEAD',
      });
      return response.ok;
    } catch {
      return false;
    }
  },
};

export default api;
