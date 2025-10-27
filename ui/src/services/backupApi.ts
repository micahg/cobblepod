import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

export interface UploadBackupResponse {
  success: boolean;
  message: string;
  file_id?: string;
  job_id?: string;
  error?: string;
}

// Use absolute URL in test environment
const getBaseUrl = () => {
  // In production (Docker/K8s), use environment variable or same origin
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL;
  }
  
  // In test environment, use absolute URL
  if (typeof window !== 'undefined' && window.location.origin) {
    return `${window.location.origin}/api`;
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
  tagTypes: ['Backup'],
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
      invalidatesTags: ['Backup'],
    }),
  }),
});

export const { useUploadBackupMutation } = backupApi;
