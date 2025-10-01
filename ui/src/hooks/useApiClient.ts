import { useMemo } from 'react';
import { useAuthToken } from '../auth/hooks';
import ApiClient from '../utils/ApiClient';

export const useApiClient = (baseURL?: string) => {
  const { getToken } = useAuthToken();

  const apiClient = useMemo(() => {
    return new ApiClient({
      baseURL: baseURL || import.meta.env.VITE_API_BASE_URL || '/api',
      getToken,
    });
  }, [getToken, baseURL]);

  return apiClient;
};