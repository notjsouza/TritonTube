import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';
import { Video, VideoUpload, VideoFilters, VideoState } from '../types';
import api from '../services/api';

// Async thunks for API calls
export const fetchVideos = createAsyncThunk(
  'videos/fetchVideos',
  async (params: { page?: number; limit?: number; filters?: VideoFilters } = {}) => {
    const { page = 1, limit = 12, filters = {} } = params;
    
    return await api.fetchVideos({
      page,
      limit,
      search: filters.search,
      sortBy: filters.sortBy,
      sortOrder: filters.sortOrder,
    });
  }
);

export const uploadVideo = createAsyncThunk(
  'videos/uploadVideo',
  async (file: File, { dispatch }) => {
    // Add upload to state immediately
    const uploadId = Date.now().toString();
    dispatch(addUpload({ 
      id: uploadId, 
      file, 
      progress: 0, 
      status: 'uploading' 
    }));

    try {
      // Upload with progress tracking
      const result = await api.uploadVideo(file, (progress) => {
        dispatch(updateUploadProgress({ id: uploadId, progress }));
      });
      dispatch(updateUploadStatus({ id: uploadId, status: 'completed' }));
      return result;
    } catch (error) {
      dispatch(updateUploadStatus({ 
        id: uploadId, 
        status: 'error', 
        error: error instanceof Error ? error.message : 'Upload failed' 
      }));
      throw error;
    }
  }
);

export const deleteVideo = createAsyncThunk(
  'videos/deleteVideo',
  async (videoId: string) => {
    await api.deleteVideo(videoId);
    return videoId;
  }
);

export const fetchVideo = createAsyncThunk(
  'videos/fetchVideo',
  async (videoId: string) => {
    return await api.getVideo(videoId);
  }
);

const initialState: VideoState = {
  videos: [],
  loading: false,
  error: null,
  currentVideo: null,
  uploads: [],
  filters: {
    search: '',
    sortBy: 'uploadTime',
    sortOrder: 'desc',
  },
  pagination: {
    page: 1,
    limit: 12,
    total: 0,
    hasMore: false,
  },
};

const videoSlice = createSlice({
  name: 'videos',
  initialState,
  reducers: {
    setCurrentVideo: (state, action: PayloadAction<Video | null>) => {
      state.currentVideo = action.payload;
    },
    setFilters: (state, action: PayloadAction<Partial<VideoFilters>>) => {
      state.filters = { ...state.filters, ...action.payload };
    },
    clearError: (state) => {
      state.error = null;
    },
    addUpload: (state, action: PayloadAction<VideoUpload>) => {
      state.uploads.push(action.payload);
    },
    updateUploadProgress: (state, action: PayloadAction<{ id: string; progress: number }>) => {
      const upload = state.uploads.find((u: VideoUpload) => u.id === action.payload.id);
      if (upload) {
        upload.progress = action.payload.progress;
      }
    },
    updateUploadStatus: (state, action: PayloadAction<{ 
      id: string; 
      status: VideoUpload['status']; 
      error?: string 
    }>) => {
      const upload = state.uploads.find((u: VideoUpload) => u.id === action.payload.id);
      if (upload) {
        upload.status = action.payload.status;
        if (action.payload.error) {
          upload.error = action.payload.error;
        }
      }
    },
    removeUpload: (state, action: PayloadAction<string>) => {
      state.uploads = state.uploads.filter((u: VideoUpload) => u.id !== action.payload);
    },
  },
  extraReducers: (builder) => {
    builder
      // Fetch videos
      .addCase(fetchVideos.pending, (state) => {
        state.loading = true;
        state.error = null;
      })
      .addCase(fetchVideos.fulfilled, (state, action) => {
        state.loading = false;
        state.videos = action.payload.data || [];
        state.pagination = {
          page: action.payload.page || 1,
          limit: action.payload.limit || 12,
          total: action.payload.total || 0,
          hasMore: action.payload.hasMore || false,
        };
      })
      .addCase(fetchVideos.rejected, (state, action) => {
        state.loading = false;
        state.error = action.error.message || 'Failed to fetch videos';
      })
      // Upload video
      .addCase(uploadVideo.fulfilled, (state, action) => {
        // Add the new video to the list
        if (action.payload) {
          state.videos.unshift(action.payload);
        }
      })
      // Delete video
      .addCase(deleteVideo.fulfilled, (state, action) => {
        state.videos = state.videos.filter((video: Video) => video.id !== action.payload);
      })
      .addCase(deleteVideo.rejected, (state, action) => {
        state.error = action.error.message || 'Failed to delete video';
      })
      // Fetch individual video
      .addCase(fetchVideo.fulfilled, (state, action) => {
        if (action.payload) {
          state.currentVideo = action.payload;
        }
      })
      .addCase(fetchVideo.rejected, (state, action) => {
        state.error = action.error.message || 'Failed to fetch video';
      });
  },
});

export const {
  setCurrentVideo,
  setFilters,
  clearError,
  addUpload,
  updateUploadProgress,
  updateUploadStatus,
  removeUpload,
} = videoSlice.actions;

export default videoSlice.reducer;
