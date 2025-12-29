import { render, screen } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { backupApi, useGetJobsQuery } from '../../services/backupApi';
import JobDashboard from './JobDashboard';
import { vi, Mock } from 'vitest';
import React from 'react';

// Mock the API module
vi.mock('../../services/backupApi', async () => {
  const actual = await vi.importActual('../../services/backupApi');
  return {
    ...actual,
    useGetJobsQuery: vi.fn(),
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
        <JobDashboard />
      </Provider>
    );
    expect(screen.getByText(/Loading jobs.../i)).toBeInTheDocument();
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
        <JobDashboard />
      </Provider>
    );

    expect(screen.getByText('waiting')).toBeInTheDocument();
    expect(screen.getByText(/ID: job-123/)).toBeInTheDocument();
  });
});
