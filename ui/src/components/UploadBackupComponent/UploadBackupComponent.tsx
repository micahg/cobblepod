import { useState, useRef } from 'react';
import styles from './UploadBackupComponent.module.css';
import { useUploadBackupMutation } from '../../services/backupApi';

const UploadBackupComponent = () => {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadBackup, { isLoading, isSuccess, isError, error }] = useUploadBackupMutation();

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

    try {
      const response = await uploadBackup(selectedFile).unwrap();
      console.log('Upload successful:', response);
      alert(`File "${selectedFile.name}" uploaded successfully!`);
      
      // Reset form
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    } catch (err) {
      console.error('Upload failed:', err);
      alert('Upload failed. Please try again.');
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
            disabled={isLoading}
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

          {isSuccess && (
            <div className={styles.successMessage}>
              Upload completed successfully!
            </div>
          )}

          {isError && (
            <div className={styles.errorMessage}>
              Upload failed: {error && 'message' in error ? error.message : 'Unknown error'}
            </div>
          )}
        </div>

        <div className={styles.buttonSection}>
          <button
            onClick={handleUpload}
            disabled={!selectedFile || isLoading}
            className={styles.uploadButton}
          >
            {isLoading ? 'Uploading...' : 'Upload File'}
          </button>
          
          {selectedFile && !isLoading && (
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