import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

export interface UploadBackupResponse {
  message: string;
  jobId?: string;
}

// Use absolute URL in test environment
const getBaseUrl = () => {
  if (typeof window !== 'undefined' && window.location.origin) {
    return `${window.location.origin}/api`;
  }
  return '/api';
};

export const backupApi = createApi({
  reducerPath: 'backupApi',
  baseQuery: fetchBaseQuery({
    baseUrl: getBaseUrl(),
    prepareHeaders: async (headers) => {
      // Get token from Auth0 if needed
      // const token = await getAccessToken();
      // if (token) {
      //   headers.set('Authorization', `Bearer ${token}`);
      // }
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
