import { useAuth0 } from '@auth0/auth0-react';

export const useAuthToken = () => {
  const { getAccessTokenSilently, isAuthenticated } = useAuth0();

  const getToken = async (): Promise<string | null> => {
    if (!isAuthenticated) {
      return null;
    }

    try {
      const token = await getAccessTokenSilently();
      return token;
    } catch (error) {
      console.error('Error getting access token:', error);
      return null;
    }
  };

  return { getToken };
};

export const useAuthenticatedUser = () => {
  const { user, isAuthenticated, isLoading } = useAuth0();
  
  return {
    user: isAuthenticated ? user : null,
    isAuthenticated,
    isLoading,
  };
};