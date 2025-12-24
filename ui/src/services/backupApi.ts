import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import { getRuntimeConfig, isConfigLoaded } from '../config/runtime';

export interface UploadBackupResponse {
  success: boolean;
  message: string;
  file_id?: string;
  job_id?: string;
  error?: string;
}

export interface JobItem {
  id: string;
  title: string;
  status: string;
  source_url: string;
  error?: string;
  duration: number;
  offset?: number;
}

export interface Job {
  id: string;
  file_id: string;
  user_id?: string;
  filename?: string;
  created_at: string;
  fail_reason?: string;
  status: string;
  items: JobItem[];
}

export interface GetJobsResponse {
  jobs: Job[];
}

// Get the API base URL from runtime config
const getBaseUrl = () => {
  // Use runtime config if loaded
  if (isConfigLoaded()) {
    const config = getRuntimeConfig();
    if (config.apiUrl) {
      return config.apiUrl;
    }
  }
  
  // Fallback to environment variable (for development)
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL;
  }
  
  // Default fallback (development with proxy)
  return '/api';
};

// Store to access auth token getter
let tokenGetter: (() => Promise<string | null>) | null = null;

export const setTokenGetter = (getter: () => Promise<string | null>) => {
  tokenGetter = getter;
};

export const backupApi = createApi({
  reducerPath: 'backupApi',
  baseQuery: fetchBaseQuery({
    baseUrl: getBaseUrl(),
    prepareHeaders: async (headers) => {
      // Get token from Auth0
      if (tokenGetter) {
        try {
          const token = await tokenGetter();
          if (token) {
            console.log('Auth token retrieved successfully');
            headers.set('Authorization', `Bearer ${token}`);
          } else {
            console.warn('Auth token is null - user may not be authenticated');
          }
        } catch (error) {
          console.error('Failed to get auth token:', error);
        }
      } else {
        console.warn('Token getter not initialized');
      }
      return headers;
    },
  }),
  tagTypes: ['Backup', 'Jobs'],
  endpoints: (builder) => ({
    uploadBackup: builder.mutation<UploadBackupResponse, File>({
      query: (file) => {
        const formData = new FormData();
        formData.append('file', file);
        
        return {
          url: '/backup/upload',
          method: 'POST',
          body: formData,
        };
      },
      invalidatesTags: ['Backup', 'Jobs'],
    }),
    getJobs: builder.query<GetJobsResponse, void>({
      query: () => '/jobs',
      providesTags: ['Jobs'],
    }),
  }),
});

export const { useUploadBackupMutation, useGetJobsQuery } = backupApi;
