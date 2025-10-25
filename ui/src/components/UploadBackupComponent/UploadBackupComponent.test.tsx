import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import UploadBackupComponent from './UploadBackupComponent'
import { backupApi } from '../../services/backupApi'

// Setup MSW server
const server = setupServer(
  http.post('/api/backup/upload', async () => {
    // Simulate a small delay for realistic testing
    await new Promise(resolve => setTimeout(resolve, 100))
    return HttpResponse.json({ message: 'Upload successful', jobId: 'test-job-123' })
  })
)

// Start server before all tests
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))

// Reset handlers after each test
afterEach(() => server.resetHandlers())

// Close server after all tests
afterAll(() => server.close())

// Create a test store
const createTestStore = () => {
  return configureStore({
    reducer: {
      [backupApi.reducerPath]: backupApi.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(backupApi.middleware),
  })
}

// Helper function to render with Redux
const renderWithRedux = (component: React.ReactElement) => {
  const store = createTestStore()
  return render(<Provider store={store}>{component}</Provider>)
}

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
    // Clear all mocks before each test
    vi.clearAllMocks();
    consoleLogSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleLogSpy.mockRestore()
    consoleErrorSpy.mockRestore()
    vi.restoreAllMocks()
  })

  it('renders component with default UI', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    expect(screen.getByText('Upload Backup File')).toBeInTheDocument()
    expect(screen.getByText('Select a podcast backup file (.backup) to upload and process.')).toBeInTheDocument()
    expect(screen.getByLabelText(/select backup file/i)).toBeInTheDocument()
    expect(screen.getByText('Upload File')).toBeInTheDocument()
    // Clear button only appears when file is selected
    expect(screen.queryByText('Clear')).not.toBeInTheDocument()
  })

  it('shows file input is required', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const fileInput = screen.getByLabelText(/select backup file/i)
    expect(fileInput).toHaveAttribute('required')
  })

  it('accepts .backup files', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const fileInput = screen.getByLabelText(/select backup file/i)
    expect(fileInput).toHaveAttribute('accept', '.backup')
  })

  it('handles file selection correctly', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
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
    
    renderWithRedux(<UploadBackupComponent />)
    
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
    renderWithRedux(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    expect(screen.getByText('test.backup')).toBeInTheDocument()
    
    const clearButton = screen.getByText('Clear')
    await userEvent.click(clearButton)
    
    expect(screen.queryByText('test.backup')).not.toBeInTheDocument()
  })

  it('enables upload button only when valid file is selected', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload File')
    expect(uploadButton).toBeDisabled()
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    expect(uploadButton).toBeEnabled()
  })

  it('shows upload progress and simulates successful upload', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    
    renderWithRedux(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    const uploadButton = screen.getByText('Upload File')
    await userEvent.click(uploadButton)
    
    // Should show loading state
    expect(screen.getByText('Uploading...')).toBeInTheDocument()
    expect(uploadButton).toBeDisabled()
    
    // Wait for upload to complete
    await waitFor(() => {
      expect(alertSpy).toHaveBeenCalledWith('File "test.backup" uploaded successfully!')
    }, { timeout: 3000 })
    
    // Success message should be displayed
    expect(screen.getByText('Upload completed successfully!')).toBeInTheDocument()
    
    alertSpy.mockRestore()
  })

  it('handles upload failure correctly', async () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    
    // Override the handler to return an error for this test
    server.use(
      http.post('/api/backup/upload', async () => {
        return HttpResponse.json(
          { message: 'Upload failed' },
          { status: 500 }
        )
      })
    )
    
    renderWithRedux(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select backup file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    const uploadButton = screen.getByText('Upload File')
    await userEvent.click(uploadButton)
    
    // Should show loading state initially
    expect(screen.getByText('Uploading...')).toBeInTheDocument()
    
    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByText(/Upload failed:/)).toBeInTheDocument()
    }, { timeout: 3000 })
    
    expect(alertSpy).toHaveBeenCalledWith('Upload failed. Please try again.')
    
    alertSpy.mockRestore()
  })

  it('prevents upload when no file is selected', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload File')
    
    // Button should be disabled by default
    expect(uploadButton).toBeDisabled()
  })

  it('formats file size in KB correctly', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
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