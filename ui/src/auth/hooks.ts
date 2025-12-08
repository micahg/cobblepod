import { useAuth0 } from '@auth0/auth0-react';
import { getRuntimeConfig } from '../config/runtime';

export const useAuthToken = () => {
  const { getAccessTokenSilently, isAuthenticated, logout } = useAuth0();

  const getToken = async (): Promise<string | null> => {
    console.log('getToken called, isAuthenticated:', isAuthenticated);
    
    if (!isAuthenticated) {
      console.warn('User is not authenticated');
      return null;
    }

    try {
      const config = getRuntimeConfig();
      // Request token with specific audience (no offline_access needed)
      const token = await getAccessTokenSilently({
        authorizationParams: {
          audience: config.audience,
        },
      });
      console.log('Token retrieved successfully');
      return token;
    } catch (error) {
      console.error('Error getting access token, logging out:', error);
      // If we can't get a token, the session is broken - force logout
      logout({ logoutParams: { returnTo: window.location.origin } });
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