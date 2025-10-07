import React, { useEffect } from 'react';
import { Box, Typography, Container, Button } from '@mui/material';
import { Add as AddIcon } from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { useAppSelector, useAppDispatch } from '../store';
import { fetchVideos } from '../store/videoSlice';
import VideoGrid from '../components/VideoGrid';

const HomePage: React.FC = () => {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { videos, loading, error, filters } = useAppSelector((state) => state.videos);

  useEffect(() => {
    dispatch(fetchVideos({ filters }));
  }, [dispatch, filters]);

  const handleUploadClick = () => {
    navigate('/upload');
  };

  return (
    <Box>
      <Container maxWidth="xl" sx={{ mt: 4, mb: 2 }}>
        <Box sx={{ 
          display: 'flex', 
          justifyContent: 'space-between', 
          alignItems: 'center',
          mb: 3 
        }}>
          <Typography variant="h4" component="h1" gutterBottom>
            Your Videos
          </Typography>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={handleUploadClick}
            size="large"
          >
            Upload Video
          </Button>
        </Box>
      </Container>

      <VideoGrid videos={videos} loading={loading} error={error} />
    </Box>
  );
};

export default HomePage;
