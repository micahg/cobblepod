import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { backupApi, useGetJobItemsQuery } from '../../services/backupApi';
import JobItemsComponent from './JobItemsComponent';
import { vi, Mock } from 'vitest';
import React from 'react';

// Mock the API module
vi.mock('../../services/backupApi', async () => {
  const actual = await vi.importActual('../../services/backupApi');
  return {
    ...actual,
    useGetJobItemsQuery: vi.fn(),
  };
});

// Create a test store
const createTestStore = () =>
  configureStore({
    reducer: {
      [backupApi.reducerPath]: backupApi.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().concat(backupApi.middleware),
  });

describe('JobItemsComponent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
});
