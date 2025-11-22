import { useAuth0 } from '@auth0/auth0-react';

export const useAuthToken = () => {
  const { getAccessTokenSilently, isAuthenticated } = useAuth0();

  const getToken = async (): Promise<string | null> => {
    console.log('getToken called, isAuthenticated:', isAuthenticated);
    
    if (!isAuthenticated) {
      console.warn('User is not authenticated');
      return null;
    }

    try {
      // Request token with specific audience (no offline_access needed)
      const token = await getAccessTokenSilently({
        authorizationParams: {
          audience: import.meta.env.VITE_AUTH0_AUDIENCE,
        },
      });
      console.log('Token retrieved successfully');
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