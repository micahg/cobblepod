import { useAuth0 } from '@auth0/auth0-react';
import type { ReactNode } from 'react';
import LoginButton from './LoginButton';

interface AuthGuardProps {
  children: ReactNode;
  fallback?: ReactNode;
}

const AuthGuard = ({ children, fallback }: AuthGuardProps) => {
  const { isAuthenticated, isLoading, error } = useAuth0();

  if (isLoading) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        minHeight: '200px' 
      }}>
        <div>Loading authentication...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ 
        padding: '20px', 
        backgroundColor: '#f8d7da', 
        color: '#721c24',
        borderRadius: '4px',
        margin: '20px'
      }}>
        <h3>Authentication Error</h3>
        <p>{error.message}</p>
        <LoginButton />
      </div>
    );
  }

  if (!isAuthenticated) {
    return fallback || (
      <div style={{ 
        display: 'flex', 
        flexDirection: 'column',
        justifyContent: 'center', 
        alignItems: 'center', 
        minHeight: '400px',
        gap: '20px'
      }}>
        <h2>Welcome to Cobblepod</h2>
        <p>Please sign in to continue</p>
        <LoginButton />
      </div>
    );
  }

  return <>{children}</>;
};

export default AuthGuard;