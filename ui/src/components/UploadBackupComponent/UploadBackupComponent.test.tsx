import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Provider } from 'react-redux'
import { configureStore } from '@reduxjs/toolkit'
import UploadBackupComponent from './UploadBackupComponent'
import { backupApi } from '../../services/backupApi'

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
    // The label text is "Select File" in the button
    expect(screen.getByText('Select File')).toBeInTheDocument()
    expect(screen.getByText('Upload')).toBeInTheDocument()
    // Clear button only appears when file is selected
    expect(screen.queryByText('Clear')).not.toBeInTheDocument()
  })

  it('shows file input is required', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    // The input is hidden but associated with the label
    // We can find it by ID or by label text if we use getByLabelText
    // Since the label contains "Select File", getByLabelText should work
    const fileInput = screen.getByLabelText(/select file/i)
    expect(fileInput).toHaveAttribute('required')
  })

  it('accepts .backup files', () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const fileInput = screen.getByLabelText(/select file/i)
    expect(fileInput).toHaveAttribute('accept', '.backup')
  })

  it('handles file selection correctly', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    // Check that the file info elements exist
    expect(screen.getByText('test.backup')).toBeInTheDocument()
    
    // Check for presence of size info
    expect(screen.getByText(/Size:/)).toBeInTheDocument()
    expect(screen.getByText(/KB/)).toBeInTheDocument()
    
    // Check for type info
    expect(screen.getByText('application/octet-stream')).toBeInTheDocument()
  })

  it('validates file extension and shows error for invalid files', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const invalidFile = new File(['content'], 'test.txt', { type: 'text/plain' })
    const fileInput = screen.getByLabelText(/select file/i) as HTMLInputElement
    
    // Use fireEvent to ensure change event is triggered even on hidden input
    fireEvent.change(fileInput, { target: { files: [invalidFile] } })
    
    // Should show error message
    expect(await screen.findByText('Please select a .backup file')).toBeInTheDocument()
    
    // Should not show file info
    expect(screen.queryByText('test.txt')).not.toBeInTheDocument()
  })

  it('clears file selection when clear button is clicked', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    // Clear button should be visible
    const clearButton = screen.getByText('Clear')
    expect(clearButton).toBeInTheDocument()
    
    await userEvent.click(clearButton)
    
    // File info should be gone
    expect(screen.queryByText('test.backup')).not.toBeInTheDocument()
    expect(fileInput.files).toHaveLength(0)
  })

  it('enables upload button only when valid file is selected', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload')
    expect(uploadButton).toBeDisabled()
    
    const file = new File(['backup content'], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    expect(uploadButton).toBeEnabled()
  })

  it('prevents upload when no file is selected', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    const uploadButton = screen.getByText('Upload')
    
    // Button should be disabled by default
    expect(uploadButton).toBeDisabled()
  })

  it('formats file size in KB correctly', async () => {
    renderWithRedux(<UploadBackupComponent />)
    
    // 1024 bytes = 1 KB
    const content = 'a'.repeat(1024)
    const file = new File([content], 'test.backup', { type: 'application/octet-stream' })
    const fileInput = screen.getByLabelText(/select file/i) as HTMLInputElement
    
    await userEvent.upload(fileInput, file)
    
    expect(screen.getByText('1.00 KB')).toBeInTheDocument()
  })
})
