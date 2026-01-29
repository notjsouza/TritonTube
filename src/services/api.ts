import { Video, PaginatedResponse } from '../types';
import { API_BASE_URL, VIDEO_CDN_URL } from '../config/api';

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

    const data = await handleResponse<PaginatedResponse<Video>>(response);

    // Normalize manifest and thumbnail URLs to use CDN if backend returned relative paths
    data.data = data.data.map((v) => {
      // Backend returns relative paths like /content/{id}/manifest.mpd and /thumbnail/{id}
      // Our S3 keys are stored as {id}/manifest.mpd and {id}/thumbnail.jpg
      if (v.manifestUrl && v.manifestUrl.startsWith('/content/')) {
        const parts = v.manifestUrl.split('/').filter(Boolean); // ['content', '{id}', 'manifest.mpd']
        if (parts.length >= 2) {
          const id = parts[1];
          v.manifestUrl = `${VIDEO_CDN_URL}/${encodeURIComponent(id)}/manifest.mpd`;
        } else {
          v.manifestUrl = `${VIDEO_CDN_URL}${v.manifestUrl}`;
        }
      }

      if (v.thumbnailUrl && v.thumbnailUrl.startsWith('/thumbnail/')) {
        const parts = v.thumbnailUrl.split('/').filter(Boolean); // ['thumbnail', '{id}']
        if (parts.length >= 2) {
          const id = parts[1];
          v.thumbnailUrl = `${VIDEO_CDN_URL}/${encodeURIComponent(id)}/thumbnail.jpg`;
        } else {
          v.thumbnailUrl = `${VIDEO_CDN_URL}${v.thumbnailUrl}`;
        }
      }

      return v;
    });

    return data;
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

    const v = await handleResponse<Video>(response);

    if (v.manifestUrl && v.manifestUrl.startsWith('/content/')) {
      const parts = v.manifestUrl.split('/').filter(Boolean);
      if (parts.length >= 2) {
        const id = parts[1];
        v.manifestUrl = `${VIDEO_CDN_URL}/${encodeURIComponent(id)}/manifest.mpd`;
      } else {
        v.manifestUrl = `${VIDEO_CDN_URL}${v.manifestUrl}`;
      }
    }

    if (v.thumbnailUrl && v.thumbnailUrl.startsWith('/thumbnail/')) {
      const parts = v.thumbnailUrl.split('/').filter(Boolean);
      if (parts.length >= 2) {
        const id = parts[1];
        v.thumbnailUrl = `${VIDEO_CDN_URL}/${encodeURIComponent(id)}/thumbnail.jpg`;
      } else {
        v.thumbnailUrl = `${VIDEO_CDN_URL}${v.thumbnailUrl}`;
      }
    }

    return v;
  },

  /**
   * Upload a new video file
   */
  async uploadVideo(file: File, onProgress?: (progress: number) => void): Promise<Video> {
    // New flow: request a presigned PUT URL from backend, upload directly to S3, then notify backend to process
    const videoId = file.name.replace(/\.mp4$/i, '');

    // 1) Request presigned URL
    const presignResp = await fetch(`${API_BASE_URL}/api/presign-upload`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      body: JSON.stringify({ videoId, filename: file.name }),
    });
    if (!presignResp.ok) {
      const err = await presignResp.json().catch(() => ({}));
      throw new ApiError(presignResp.status, err.message || 'Failed to get presigned URL');
    }
    const presignData = await presignResp.json();
    const uploadUrl: string = presignData.url;

    // 2) Upload the file directly to S3 with PUT
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('PUT', uploadUrl);

      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable && onProgress) {
          const percentComplete = (event.loaded / event.total) * 100;
          onProgress(percentComplete);
        }
      });

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve();
        } else {
          reject(new ApiError(xhr.status, `Upload failed: ${xhr.statusText}`));
        }
      });

      xhr.addEventListener('error', () => reject(new ApiError(0, 'Network error occurred')));
      xhr.addEventListener('abort', () => reject(new ApiError(0, 'Upload aborted')));

      xhr.send(file);
    });

    // 3) Notify backend to start processing the uploaded s3 object
    const notifyResp = await fetch(`${API_BASE_URL}/api/process`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      body: JSON.stringify({ videoId, filename: file.name }),
    });

    if (!notifyResp.ok) {
      const err = await notifyResp.json().catch(() => ({}));
      throw new ApiError(notifyResp.status, err.message || 'Failed to start processing');
    }

    // Return a small object indicating processing started (client should poll /api/videos)
    const data = await notifyResp.json();
    return data as Video;
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
