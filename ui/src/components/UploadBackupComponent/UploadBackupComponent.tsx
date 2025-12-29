import { useState, useRef } from 'react';
import { useUploadBackupMutation } from '../../services/backupApi';
import { 
  Card, 
  CardContent, 
  Typography, 
  Button, 
  Box, 
  Alert, 
  CircularProgress,
  Stack
} from '@mui/material';
import CloudUploadIcon from '@mui/icons-material/CloudUpload';

const UploadBackupComponent = () => {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadBackup, { isLoading }] = useUploadBackupMutation();
  const [uploadMessage, setUploadMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null);

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      // Validate file extension
      if (!file.name.toLowerCase().endsWith('.backup')) {
        setUploadMessage({ type: 'error', text: 'Please select a .backup file' });
        return;
      }
      setSelectedFile(file);
      setUploadMessage(null);
    }
  };

  const handleUpload = async () => {
    if (!selectedFile) {
      setUploadMessage({ type: 'error', text: 'Please select a file first' });
      return;
    }

    try {
      const response = await uploadBackup(selectedFile).unwrap();
      console.log('Upload successful:', response);
      
      if (response.success) {
        const message = response.job_id 
          ? `File "${selectedFile.name}" uploaded successfully! Job ID: ${response.job_id}`
          : `File "${selectedFile.name}" uploaded successfully!`;
        setUploadMessage({ type: 'success', text: message });
      } else {
        setUploadMessage({ type: 'error', text: `Upload failed: ${response.error || 'Unknown error'}` });
      }
      
      // Reset form on success
      if (response.success) {
        setSelectedFile(null);
        if (fileInputRef.current) {
          fileInputRef.current.value = '';
        }
      }
    } catch (err) {
      console.error('Upload failed:', err);
      const errorMessage = err && typeof err === 'object' && 'data' in err 
        ? (err.data as { error?: string })?.error || 'Unknown error'
        : 'Network error';
      setUploadMessage({ type: 'error', text: `Upload failed: ${errorMessage}` });
    }
  };

  const handleClear = () => {
    setSelectedFile(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
    setUploadMessage(null);
  };

  return (
    <Card sx={{ minWidth: 300, maxWidth: 600, width: '100%', maxHeight: 600, overflow: 'auto' }}>
      <CardContent>
        <Typography variant="h5" component="div" gutterBottom>
          Upload Backup File
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Select a podcast backup file (.backup) to upload and process.
        </Typography>
        
        <Stack spacing={2}>
          <Box>
            <input
              accept=".backup"
              style={{ display: 'none' }}
              id="raised-button-file"
              type="file"
              onChange={handleFileSelect}
              ref={fileInputRef}
              disabled={isLoading}
              required
            />
            <label htmlFor="raised-button-file">
              <Button 
                variant="outlined" 
                component="span" 
                startIcon={<CloudUploadIcon />}
                fullWidth
                disabled={isLoading}
              >
                Select File
              </Button>
            </label>
          </Box>
          
          {selectedFile && (
            <Box sx={{ p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
              <Typography variant="body2"><strong>Selected file:</strong> {selectedFile.name}</Typography>
              <Typography variant="body2"><strong>Size:</strong> {(selectedFile.size / 1024).toFixed(2)} KB</Typography>
              <Typography variant="body2"><strong>Type:</strong> {selectedFile.type || 'Unknown'}</Typography>
            </Box>
          )}

          {uploadMessage && (
            <Alert severity={uploadMessage.type} onClose={() => setUploadMessage(null)}>
              {uploadMessage.text}
            </Alert>
          )}

          <Stack direction="row" spacing={2}>
            <Button 
              variant="contained" 
              color="primary" 
              onClick={handleUpload}
              disabled={!selectedFile || isLoading}
              fullWidth
            >
              {isLoading ? <CircularProgress size={24} /> : 'Upload'}
            </Button>
            {selectedFile && !isLoading && (
              <Button 
                variant="outlined" 
                color="secondary" 
                onClick={handleClear}
                fullWidth
              >
                Clear
              </Button>
            )}
          </Stack>
        </Stack>
      </CardContent>
    </Card>
  );
};

export default UploadBackupComponent;