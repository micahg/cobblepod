import { render, screen, waitFor } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import { backupApi } from '../../services/backupApi';
import JobDashboard from './JobDashboard';
import { vi } from 'vitest';
import React from 'react';

// Mock the API
const mockGetJobs = vi.fn();

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
  it('renders loading state initially', () => {
    const store = createTestStore();
    render(
      <Provider store={store}>
        <JobDashboard />
      </Provider>
    );
    expect(screen.getByText(/Loading jobs.../i)).toBeInTheDocument();
  });
});
