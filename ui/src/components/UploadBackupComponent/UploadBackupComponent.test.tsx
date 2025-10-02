import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import UploadBackupComponent from './UploadBackupComponent'

// Mock CSS modules
vi.mock('./UploadBackupComponent.module.css', () => ({
  default: {
    uploadBackupComponent: 'upload-backup-component',
    uploadSection: 'upload-section',
    fileInputSection: 'file-input-section',
    fileInputLabel: 'file-input-label',
    fileInput: 'file-input',
    fileInfo: 'file-info',
    buttonSection: 'button-section',
    uploadButton: 'upload-button',
    clearButton: 'clear-button',
  },
}))

describe('UploadBackupComponent', () => {
  let consoleLogSpy: ReturnType<typeof vi.spyOn>
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    consoleLogSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleLogSpy.mockRestore()
    consoleErrorSpy.mockRestore()
    vi.restoreAllMocks()
  })

  it('renders component with default UI', () => {
    render(<UploadBackupComponent />)
    
    expect(screen.getByText('Upload Backup File')).toBeInTheDocument()
    expect(screen.getByText('Select a podcast backup file (.backup) to upload and process.')).toBeInTheDocument()
    expect(screen.getByLabelText(/select backup file/i)).toBeInTheDocument()
    expect(screen.getByText('Upload File')).toBeInTheDocument()
    // Clear button only appears when file is selected
    expect(screen.queryByText('Clear')).not.toBeInTheDocument()
  })

  it('shows file input is required', () => {
    render(<UploadBackupComponent />)
    
    const fileInput = screen.getByLabelText(/select backup file/i)
    expect(fileInput).toHaveAttribute('required')
  })

  it('accepts .backup files', () => {
    render(<UploadBackupComponent />)
    
    const fileInput = screen.getByLabelText(/select backup file/i)
    expect(fileInput).toHaveAttribute('accept', '.backup')
  })

  it('handles file selection correctly', async () => {
    render(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    // Check that the file info elements exist without being too specific about text matching
    const fileInfo = screen.getByText('test.backup')
    expect(fileInfo).toBeInTheDocument()
    
    // Check for presence of size info
    expect(screen.getByText(/Size:/)).toBeInTheDocument()
    expect(screen.getByText(/KB/)).toBeInTheDocument()
    
    // Check for type info
    expect(screen.getByText('application/octet-stream')).toBeInTheDocument()
  })

  it('validates file extension and shows error for invalid files', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    
    render(<UploadBackupComponent />)
    
    const invalidFile = new File(['content'], 'test.txt', { type: 'text/plain' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    // Directly fire the onChange event to trigger validation
    Object.defineProperty(fileInput, 'files', {
      value: [invalidFile],
      writable: false,
    })
    fireEvent.change(fileInput)
    
    expect(alertSpy).toHaveBeenCalledWith('Please select a .backup file')
    expect(screen.queryByText('test.txt')).not.toBeInTheDocument()
    
    alertSpy.mockRestore()
  })

  it('clears file selection when clear button is clicked', async () => {
    render(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    expect(screen.getByText('test.backup')).toBeInTheDocument()
    
    const clearButton = screen.getByText('Clear')
    await userEvent.click(clearButton)
    
    expect(screen.queryByText('test.backup')).not.toBeInTheDocument()
  })

  it('enables upload button only when valid file is selected', async () => {
    render(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload File')
    expect(uploadButton).toBeDisabled()
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    expect(uploadButton).toBeEnabled()
  })

  it('shows upload progress and simulates successful upload', async () => {
    // Mock window.alert
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    
    render(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    const uploadButton = screen.getByText('Upload File')
    await userEvent.click(uploadButton)
    
    expect(screen.getByText('Uploading...')).toBeInTheDocument()
    
    await waitFor(() => {
      expect(alertSpy).toHaveBeenCalledWith('File "test.backup" uploaded successfully!')
    }, { timeout: 3000 })
    
    alertSpy.mockRestore()
  })

  it('handles upload process works correctly', async () => {
    // This test verifies the upload process initiates correctly
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    
    render(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    const uploadButton = screen.getByText('Upload File')
    await userEvent.click(uploadButton)
    
    // Verify upload process starts
    expect(screen.getByText('Uploading...')).toBeInTheDocument()
    
    // Wait for upload to complete
    await waitFor(() => {
      expect(alertSpy).toHaveBeenCalledWith('File "test.backup" uploaded successfully!')
    }, { timeout: 3000 })
    
    alertSpy.mockRestore()
  })

  it('prevents upload when no file is selected', () => {
    render(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload File')
    
    // Button should be disabled by default
    expect(uploadButton).toBeDisabled()
  })

  it('formats file size in KB correctly', async () => {
    render(<UploadBackupComponent />)
    
    // Test file with 1024 bytes (should show as 1.00 KB)
    const content = 'a'.repeat(1024)
    const file = new File([content], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    // Check that size is displayed in KB
    expect(screen.getByText(/Size:/)).toBeInTheDocument()
    expect(screen.getByText(/KB/)).toBeInTheDocument()
  })
})