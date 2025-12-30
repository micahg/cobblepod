import { lazy, Suspense, useEffect } from 'react'
import { Provider } from 'react-redux'
import { store } from './store/store'
import { AuthGuard, LogoutButton, useAuthToken } from './auth'
import { setTokenGetter } from './services/backupApi'
import { Container, Typography, Box, CircularProgress, CssBaseline, Stack } from '@mui/material'

// Lazy load the UploadBackupComponent
const UploadBackupComponent = lazy(() => import('./components/UploadBackupComponent/UploadBackupComponent'))
const JobDashboardComponent = lazy(() => import('./components/JobDashboardComponent/JobDashboardComponent'))

function AppContent() {
  const { getToken } = useAuthToken()

  // Set up token getter for RTK Query
  useEffect(() => {
    const initializeToken = async () => {
      console.log('Setting token getter');
      setTokenGetter(getToken)
      
      // Test the token getter immediately
      try {
        const token = await getToken();
        if (token) {
          console.log('Token retrieved successfully, length:', token.length);
        } else {
          console.warn('Token is null - check Auth0 configuration');
        }
      } catch (err) {
        console.error('Error getting token:', err);
      }
    };
    
    initializeToken();
  }, [getToken])

  return (
    <Container 
      maxWidth="lg"
      sx={{ 
        display: 'flex', 
        flexDirection: 'row', 
        justifyContent: 'center', 
        alignItems: 'center',
        minHeight: '100vh', 
        minWidth: '100vw'
      }}
    >
      <Box sx={{ my: 4, textAlign: 'center', width: '100%' }}>
        <Typography variant="h3" component="h1" gutterBottom>
          Cobblepod Dashboard
        </Typography>

        <Stack 
          direction={{ xs: 'column', md: 'row' }} 
          spacing={4} 
          justifyContent="center" 
          alignItems="flex-start"
          sx={{ mt: 4 }}
        >
          {/* Upload Backup Component */}
          <Suspense fallback={<CircularProgress />}>
            <UploadBackupComponent />
          </Suspense>

          {/* Job Dashboard Component */}
          <Suspense fallback={<CircularProgress />}>
            <JobDashboardComponent />
          </Suspense>
        </Stack>
        
        <Box sx={{ mt: 4 }}>
          <LogoutButton />
        </Box>
      </Box>
    </Container>
  )
}

function App() {
  return (
    <Provider store={store}>
      <CssBaseline />
      <AuthGuard>
        <AppContent />
      </AuthGuard>
    </Provider>
  )
}

export default App
