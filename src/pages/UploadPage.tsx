import React, { useCallback, useState } from 'react';
import {
  Box,
  Typography,
  Container,
  Button,
  Paper,
  LinearProgress,
  Alert,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  IconButton,
  Card,
  CardContent,
} from '@mui/material';
import {
  CloudUpload as CloudUploadIcon,
  Delete as DeleteIcon,
  CheckCircle as CheckCircleIcon,
  Error as ErrorIcon,
  ArrowBack as ArrowBackIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { useAppSelector, useAppDispatch } from '../store';
import { uploadVideo, removeUpload } from '../store/videoSlice';

const UploadPage: React.FC = () => {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const { uploads } = useAppSelector((state) => state.videos);
  const [dragOver, setDragOver] = useState(false);

  const handleFileSelect = useCallback((files: FileList | null) => {
    if (!files) return;

    Array.from(files).forEach((file) => {
      if (file.type === 'video/mp4') {
        dispatch(uploadVideo(file));
      } else {
        alert('Please select only MP4 video files');
      }
    });
  }, [dispatch]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    handleFileSelect(e.dataTransfer.files);
  }, [handleFileSelect]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  }, []);

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    handleFileSelect(e.target.files);
  };

  const handleRemoveUpload = (uploadId: string) => {
    dispatch(removeUpload(uploadId));
  };

  const handleBackClick = () => {
    navigate('/');
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircleIcon color="success" />;
      case 'error':
        return <ErrorIcon color="error" />;
      default:
        return null;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'success';
      case 'error':
        return 'error';
      default:
        return 'primary';
    }
  };

  return (
    <Container maxWidth="md" sx={{ mt: 3, mb: 4 }}>
      <Button
        startIcon={<ArrowBackIcon />}
        onClick={handleBackClick}
        sx={{ mb: 3 }}
      >
        Back to Home
      </Button>

      <Typography variant="h4" component="h1" gutterBottom>
        Upload Videos
      </Typography>

      <Card sx={{ mb: 4 }}>
        <CardContent>
          <Paper
            sx={{
              p: 4,
              textAlign: 'center',
              border: '2px dashed',
              borderColor: dragOver ? 'primary.main' : 'grey.300',
              backgroundColor: dragOver ? 'action.hover' : 'background.paper',
              cursor: 'pointer',
              transition: 'all 0.2s ease-in-out',
              '&:hover': {
                borderColor: 'primary.main',
                backgroundColor: 'action.hover',
              },
            }}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onClick={() => document.getElementById('file-input')?.click()}
          >
            <CloudUploadIcon sx={{ fontSize: 64, color: 'primary.main', mb: 2 }} />
            <Typography variant="h6" gutterBottom>
              Drag and drop MP4 files here
            </Typography>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              or click to browse files
            </Typography>
            <input
              id="file-input"
              type="file"
              accept="video/mp4"
              multiple
              onChange={handleFileInput}
              style={{ display: 'none' }}
            />
            <Button
              variant="contained"
              startIcon={<CloudUploadIcon />}
              sx={{ mt: 2 }}
            >
              Select Files
            </Button>
          </Paper>
        </CardContent>
      </Card>

      {uploads.length > 0 && (
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Upload Queue
            </Typography>
            
            <List>
              {uploads.map((upload) => (
                <ListItem key={upload.id} divider>
                  <ListItemText
                    primary={upload.file.name}
                    secondary={
                      <Box>
                        <Typography variant="body2" color="text.secondary">
                          {formatFileSize(upload.file.size)} • {upload.status}
                        </Typography>
                        {upload.status === 'uploading' && (
                          <LinearProgress
                            variant="determinate"
                            value={upload.progress}
                            color={getStatusColor(upload.status) as any}
                            sx={{ mt: 1 }}
                          />
                        )}
                        {upload.error && (
                          <Alert severity="error" sx={{ mt: 1 }}>
                            {upload.error}
                          </Alert>
                        )}
                      </Box>
                    }
                  />
                  <ListItemSecondaryAction>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      {getStatusIcon(upload.status)}
                      <IconButton
                        edge="end"
                        aria-label="remove"
                        onClick={() => handleRemoveUpload(upload.id)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </Box>
                  </ListItemSecondaryAction>
                </ListItem>
              ))}
            </List>
          </CardContent>
        </Card>
      )}
    </Container>
  );
};

export default UploadPage;
