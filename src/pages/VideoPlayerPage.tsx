import React, { useEffect, useRef } from 'react';
import {
  Box,
  Typography,
  Container,
  Button,
  Paper,
  Alert,
  CircularProgress,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { useParams, useNavigate } from 'react-router-dom';
import { useAppSelector, useAppDispatch } from '../store';
import { setCurrentVideo, fetchVideo } from '../store/videoSlice';

const VideoPlayerPage: React.FC = () => {
  const { videoId } = useParams<{ videoId: string }>();
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const videoRef = useRef<HTMLVideoElement>(null);
  const playerRef = useRef<any>(null);
  
  const { currentVideo, videos, loading } = useAppSelector((state) => state.videos);

  useEffect(() => {
    if (videoId) {
      // Find video in existing list first
      const video = videos.find(v => v.escapedId === videoId || v.id === videoId);
      if (video) {
        dispatch(setCurrentVideo(video));
      } else {
        // Fetch individual video if not found in list
        dispatch(fetchVideo(videoId));
      }
    }
  }, [videoId, videos, dispatch]);

  useEffect(() => {
    if (currentVideo && videoRef.current) {
      // Initialize DASH player when video is loaded
      const initPlayer = async () => {
        try {
          // Dynamically import dashjs
          const { MediaPlayer } = await import('dashjs');
          
          // Use manifestUrl from video object or construct it
          const url = currentVideo.manifestUrl || `/content/${currentVideo.id}/manifest.mpd`;
          
          if (playerRef.current) {
            playerRef.current.destroy();
          }
          
          playerRef.current = MediaPlayer().create();
          playerRef.current.initialize(videoRef.current, url, true);
          
          // Configure player for better streaming
          playerRef.current.updateSettings({
            streaming: {
              abr: {
                autoSwitchBitrate: { video: true, audio: true }
              },
              buffer: {
                fastSwitchEnabled: true
              }
            }
          });
        } catch (error) {
          console.error('Failed to initialize DASH player:', error);
        }
      };

      initPlayer();
    }

    return () => {
      if (playerRef.current) {
        playerRef.current.destroy();
        playerRef.current = null;
      }
    };
  }, [currentVideo]);

  const handleBackClick = () => {
    navigate('/');
  };

  const formatDate = (dateString: string) => {
    try {
      return new Date(dateString).toLocaleString();
    } catch {
      return dateString;
    }
  };

  if (loading) {
    return (
      <Container maxWidth="lg" sx={{ mt: 4, textAlign: 'center' }}>
        <CircularProgress />
      </Container>
    );
  }

  if (!currentVideo) {
    return (
      <Container maxWidth="lg" sx={{ mt: 4 }}>
        <Alert severity="error">
          Video not found
        </Alert>
        <Button
          startIcon={<ArrowBackIcon />}
          onClick={handleBackClick}
          sx={{ mt: 2 }}
        >
          Back to Home
        </Button>
      </Container>
    );
  }

  return (
    <Container maxWidth="lg" sx={{ mt: 3, mb: 4 }}>
      <Button
        startIcon={<ArrowBackIcon />}
        onClick={handleBackClick}
        sx={{ mb: 2 }}
      >
        Back to Home
      </Button>

      <Paper elevation={1} sx={{ p: 0, overflow: 'hidden' }}>
        <Box sx={{ position: 'relative', width: '100%', paddingTop: '56.25%' }}>
          <video
            ref={videoRef}
            controls
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: '100%',
              backgroundColor: '#000',
            }}
          />
        </Box>
      </Paper>

      <Box sx={{ mt: 3 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          {currentVideo.title || currentVideo.id}
        </Typography>
        
        <Typography variant="body2" color="text.secondary" gutterBottom>
          Uploaded: {formatDate(currentVideo.uploadTime || currentVideo.uploadedAt)}
        </Typography>

        {currentVideo.description && (
          <Typography variant="body1" sx={{ mt: 2 }}>
            {currentVideo.description}
          </Typography>
        )}

        {currentVideo.fileSize && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            File size: {(currentVideo.fileSize / (1024 * 1024)).toFixed(2)} MB
          </Typography>
        )}
      </Box>
    </Container>
  );
};

export default VideoPlayerPage;
