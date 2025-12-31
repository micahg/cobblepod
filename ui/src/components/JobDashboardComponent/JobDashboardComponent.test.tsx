import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { backupApi, useGetJobsQuery, useGetJobItemsQuery } from '../../services/backupApi';
import JobDashboardComponent from './JobDashboardComponent';
import { vi, Mock } from 'vitest';
import React from 'react';

// Mock the API module
vi.mock('../../services/backupApi', async () => {
  const actual = await vi.importActual('../../services/backupApi');
  return {
    ...actual,
    useGetJobsQuery: vi.fn(),
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

describe('JobDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined,
    });
  });

  it('renders loading state initially', () => {
    (useGetJobsQuery as Mock).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );
    // In MUI version, we use CircularProgress, so we look for role="progressbar"
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders waiting job correctly', () => {
    const mockJob = {
      id: 'job-123',
      file_id: 'file-123',
      created_at: new Date().toISOString(),
      status: 'waiting',
      items: []
    };

    (useGetJobsQuery as Mock).mockReturnValue({
      data: { jobs: [mockJob] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );

    expect(screen.getByText(/ID: job-123/)).toBeInTheDocument();
    expect(screen.getByLabelText('info')).toBeInTheDocument();
  });

  it('displays no active jobs message when list is empty', () => {
    (useGetJobsQuery as Mock).mockReturnValue({
      data: { jobs: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );

    expect(screen.getByText('No active jobs found.')).toBeInTheDocument();
  });

  it('displays no completed jobs message when list is empty', () => {
    (useGetJobsQuery as Mock).mockReturnValue({
      data: { jobs: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );

    const completedTab = screen.getByRole('tab', { name: /completed/i });
    fireEvent.click(completedTab);

    expect(screen.getByText('No completed jobs found.')).toBeInTheDocument();
  });

  it('displays no failed jobs message when list is empty', () => {
    (useGetJobsQuery as Mock).mockReturnValue({
      data: { jobs: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );

    const failedTab = screen.getByRole('tab', { name: /failed/i });
    fireEvent.click(failedTab);

    expect(screen.getByText('No failed jobs found.')).toBeInTheDocument();
  });

  it('opens job details when info button is clicked', () => {
    const mockJob = {
      id: 'job-123',
      file_id: 'file-123',
      created_at: new Date().toISOString(),
      status: 'waiting',
      items: []
    };

    (useGetJobsQuery as Mock).mockReturnValue({
      data: { jobs: [mockJob] },
      isLoading: false,
      error: undefined,
    });

    (useGetJobItemsQuery as Mock).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      error: undefined,
    });

    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboardComponent />
      </Provider>
    );

    const infoButton = screen.getByLabelText('info');
    fireEvent.click(infoButton);

    expect(screen.getByText(/Job Details/)).toBeInTheDocument();
    // Check that the job ID is displayed in the dialog title
    expect(screen.getByText(/Job Details.*job-123/)).toBeInTheDocument();
  });
});
