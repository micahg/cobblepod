import { lazy, Suspense, useEffect } from 'react'
import { Provider } from 'react-redux'
import { store } from './store/store'
import { AuthGuard, AccountMenu, useAuthToken } from './auth'
import { setTokenGetter } from './services/api'
import { AppBar, Toolbar, Container, Typography, CircularProgress, CssBaseline, Stack } from '@mui/material'

// Lazy load the UploadBackupComponent
const UploadBackupComponent = lazy(() => import('./components/UploadBackupComponent/UploadBackupComponent'))
const JobDashboardComponent = lazy(() => import('./components/JobDashboardComponent/JobDashboardComponent'))
const RSSComponent = lazy(() => import('./components/RSSComponent/RSSComponent'))

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
    <>
      <AppBar position="sticky" color="default" elevation={0}>
        <Toolbar>
          <Typography variant="h6" component="h1" sx={{ flex: 1 }}>
            Cobblepod Dashboard
          </Typography>
          <AccountMenu />
        </Toolbar>
      </AppBar>

      <Container
        maxWidth="lg"
        sx={{
          display: 'flex',
          flexDirection: 'row',
          justifyContent: 'center',
          alignItems: 'flex-start',
          minHeight: 'calc(100vh - 64px)',
          minWidth: '100vw',
          py: 4,
        }}
      >
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={4}
          justifyContent="center"
          alignItems="flex-start"
        >
          <Stack spacing={4} sx={{ width: '100%', maxWidth: 500 }}>
            {/* RSS Component */}
            <Suspense fallback={<CircularProgress />}>
              <RSSComponent />
            </Suspense>

            {/* Upload Backup Component */}
            <Suspense fallback={<CircularProgress />}>
              <UploadBackupComponent />
            </Suspense>
          </Stack>

          {/* Job Dashboard Component */}
          <Suspense fallback={<CircularProgress />}>
            <JobDashboardComponent />
          </Suspense>
        </Stack>
      </Container>
    </>
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
