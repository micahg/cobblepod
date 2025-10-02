import { useState, useRef } from 'react';
import styles from './UploadBackupComponent.module.css';

const UploadBackupComponent = () => {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      // Validate file extension
      if (!file.name.toLowerCase().endsWith('.backup')) {
        alert('Please select a .backup file');
        return;
      }
      setSelectedFile(file);
    }
  };

  const handleUpload = async () => {
    if (!selectedFile) {
      alert('Please select a file first');
      return;
    }

    setIsUploading(true);
    
    try {
      // TODO: Connect with API later
      console.log('Uploading file:', selectedFile.name);
      console.log('File size:', selectedFile.size, 'bytes');
      
      // Simulate upload delay
      await new Promise(resolve => setTimeout(resolve, 2000));
      
      alert(`File "${selectedFile.name}" uploaded successfully!`);
      
      // Reset form
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    } catch (error) {
      console.error('Upload failed:', error);
      alert('Upload failed. Please try again.');
    } finally {
      setIsUploading(false);
    }
  };

  const handleClear = () => {
    setSelectedFile(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  return (
    <div className={styles.uploadBackupComponent}>
      <h2>Upload Backup File</h2>
      <p>Select a podcast backup file (.backup) to upload and process.</p>
      
      <div className={styles.uploadSection}>
        <div className={styles.fileInputSection}>
          <label htmlFor="backup-file-input" className={styles.fileInputLabel}>
            Select backup file:
          </label>
          <input
            id="backup-file-input"
            ref={fileInputRef}
            type="file"
            accept=".backup"
            onChange={handleFileSelect}
            disabled={isUploading}
            required
            className={styles.fileInput}
          />
          
          {selectedFile && (
            <div className={styles.fileInfo}>
              <p><strong>Selected file:</strong> {selectedFile.name}</p>
              <p><strong>Size:</strong> {(selectedFile.size / 1024).toFixed(2)} KB</p>
              <p><strong>Type:</strong> {selectedFile.type || 'Unknown'}</p>
            </div>
          )}
        </div>

        <div className={styles.buttonSection}>
          <button
            onClick={handleUpload}
            disabled={!selectedFile || isUploading}
            className={styles.uploadButton}
          >
            {isUploading ? 'Uploading...' : 'Upload File'}
          </button>
          
          {selectedFile && !isUploading && (
            <button
              onClick={handleClear}
              className={styles.clearButton}
            >
              Clear
            </button>
          )}
        </div>
      </div>

    </div>
  );
};

export default UploadBackupComponent;