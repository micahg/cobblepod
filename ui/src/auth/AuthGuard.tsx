import { useAuth0 } from '@auth0/auth0-react';
import { Box, CircularProgress, Typography, Alert, Container } from '@mui/material';
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
      <Box 
        display="flex" 
        justifyContent="center" 
        alignItems="center" 
        minHeight="200px"
      >
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Container maxWidth="sm" sx={{ mt: 4 }}>
        <Alert severity="error" sx={{ mb: 2 }}>
          <Typography variant="h6">Authentication Error</Typography>
          <Typography variant="body2">{error.message}</Typography>
        </Alert>
        <Box display="flex" justifyContent="center">
          <LoginButton />
        </Box>
      </Container>
    );
  }

  if (!isAuthenticated) {
    return fallback || (
      <Container 
        // maxWidth="sm" 
        sx={{ 
          display: 'flex', 
          flexDirection: 'row', 
          justifyContent: 'center', 
          alignItems: 'center',
          minHeight: '100vh', 
          minWidth: '100vw'
        }}
      >
        <Box 
          display="flex" 
          flexDirection="column"
          alignItems="center" 
          gap={3}
        >
          <Typography variant="h4" component="h2" gutterBottom align="center">
            Welcome to Cobblepod
          </Typography>
          <Typography variant="body1" color="text.secondary" gutterBottom align="center">
            Please sign in to continue
          </Typography>
          <LoginButton />
        </Box>
      </Container>
    );
  }

  return <>{children}</>;
};

export default AuthGuard;