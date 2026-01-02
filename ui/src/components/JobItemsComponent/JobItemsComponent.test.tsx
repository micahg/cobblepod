import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { api, useGetJobItemsQuery, useCancelJobMutation } from '../../services/api';
import JobItemsComponent from './JobItemsComponent';
import { vi, Mock } from 'vitest';
import React from 'react';

// Mock the API module
vi.mock('../../services/api', async () => {
  const actual = await vi.importActual('../../services/api');
  return {
    ...actual,
    useGetJobItemsQuery: vi.fn(),
    useCancelJobMutation: vi.fn(),
  };
});

// Create a test store
const createTestStore = () =>
  configureStore({
    reducer: {
      [api.reducerPath]: api.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(api.middleware),
  });

describe('JobItemsComponent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (useCancelJobMutation as Mock).mockReturnValue([vi.fn(), { isLoading: false }]);
  });

  it('renders nothing when closed', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={false} onClose={() => {}} />
      </Provider>
    );
    
    expect(screen.queryByText(/Job Details/)).not.toBeInTheDocument();
  });

  it('renders loading state', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={() => {}} />
      </Provider>
    );
    
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders items correctly', () => {
    const mockItems = [
      {
        id: 'item-1',
        title: 'Episode 1',
        status: 'completed',
        source_url: 'http://example.com/1.mp3',
        duration: 60000000000, // 60s
      },
      {
        id: 'item-2',
        title: 'Episode 2',
        status: 'failed',
        source_url: 'http://example.com/2.mp3',
        error: 'Download failed',
        duration: 0,
      }
    ];

    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: mockItems },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={() => {}} />
      </Provider>
    );

    expect(screen.getByText('Episode 1')).toBeInTheDocument();
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('Episode 2')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByText('Error: Download failed')).toBeInTheDocument();
  });

  it('calls onClose when close button is clicked', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const onClose = vi.fn();
    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={onClose} />
      </Provider>
    );

    const closeButton = screen.getByLabelText('close');
    fireEvent.click(closeButton);
    expect(onClose).toHaveBeenCalled();
  });

  it('calls cancelJob when cancel button is clicked', async () => {
    const mockCancelJob = vi.fn().mockReturnValue({ unwrap: vi.fn().mockResolvedValue({}) });
    (useCancelJobMutation as Mock).mockReturnValue([mockCancelJob, { isLoading: false }]);
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const onClose = vi.fn();
    const store = createTestStore();
    
    // Mock window.confirm
    const confirmSpy = vi.spyOn(window, 'confirm');
    confirmSpy.mockImplementation(() => true);

    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={onClose} />
      </Provider>
    );

    const cancelButton = screen.getByLabelText('cancel job');
    fireEvent.click(cancelButton);

    expect(confirmSpy).toHaveBeenCalled();
    expect(mockCancelJob).toHaveBeenCalledWith('job-123');
  });

  it('disables cancel button when job is completed', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" jobStatus="completed" open={true} onClose={() => {}} />
      </Provider>
    );

    const cancelButton = screen.getByLabelText('cancel job');
    expect(cancelButton).toBeDisabled();
  });

  it('disables cancel button when job is failed', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" jobStatus="failed" open={true} onClose={() => {}} />
      </Provider>
    );

    const cancelButton = screen.getByLabelText('cancel job');
    expect(cancelButton).toBeDisabled();
  });

  it('displays failure reason when provided', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent 
          jobId="job-123" 
          jobStatus="failed" 
          failReason="Disk full"
          open={true} 
          onClose={() => {}} 
        />
      </Provider>
    );

    expect(screen.getByText('Job Failed: Disk full')).toBeInTheDocument();
  });

  it('disables cancel button when loading', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={() => {}} />
      </Provider>
    );

    const cancelButton = screen.getByLabelText('cancel job');
    expect(cancelButton).toBeDisabled();
  });

  it('disables refresh button when loading', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" open={true} onClose={() => {}} />
      </Provider>
    );

    const refreshButton = screen.getByLabelText('refresh');
    expect(refreshButton).toBeDisabled();
  });

  it('disables refresh button when job is completed', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" jobStatus="completed" open={true} onClose={() => {}} />
      </Provider>
    );

    const refreshButton = screen.getByLabelText('refresh');
    expect(refreshButton).toBeDisabled();
  });

  it('disables refresh button when job is failed', () => {
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobItemsComponent jobId="job-123" jobStatus="failed" open={true} onClose={() => {}} />
      </Provider>
    );

    const refreshButton = screen.getByLabelText('refresh');
    expect(refreshButton).toBeDisabled();
  });
});
