import { Auth0Provider } from '@auth0/auth0-react';
import type { ReactNode } from 'react';
import { getRuntimeConfig } from '../config/runtime';

interface Auth0ProviderWrapperProps {
  children: ReactNode;
}

const Auth0ProviderWrapper = ({ children }: Auth0ProviderWrapperProps) => {
  const config = getRuntimeConfig();
  const { domain, clientId, audience } = config;

  if (!domain || !clientId) {
    console.error('Auth0 domain and client ID are required');
    return <div>Authentication configuration missing</div>;
  }

  return (
    <Auth0Provider
      domain={domain}
      clientId={clientId}
      authorizationParams={{
        redirect_uri: window.location.origin,
        audience: audience,
        scope: 'openid profile email offline_access',
        connection: 'google-oauth2', // Force Google authentication
      }}
      useRefreshTokens={true}
      cacheLocation="localstorage"
    >
      {children}
    </Auth0Provider>
  );
};

export default Auth0ProviderWrapper;