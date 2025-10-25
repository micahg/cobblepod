import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

export interface UploadBackupResponse {
  message: string;
  jobId?: string;
}

export const backupApi = createApi({
  reducerPath: 'backupApi',
  baseQuery: fetchBaseQuery({
    baseUrl: '/api',
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
