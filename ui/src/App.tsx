import { lazy, Suspense, useEffect, useState } from 'react'
import { Provider } from 'react-redux'
import { store } from './store/store'
import { AuthGuard, useAuthToken } from './auth'
import { setTokenGetter } from './services/api'
import { AppBar, Toolbar, Container, Typography, CircularProgress, CssBaseline, Stack, Box } from '@mui/material'

// Lazy load the components so their chunks load on demand.
const AccountMenu = lazy(() => import('./auth/AccountMenu'))
const UploadBackupComponent = lazy(() => import('./components/UploadBackupComponent/UploadBackupComponent'))
const JobDashboardComponent = lazy(() => import('./components/JobDashboardComponent/JobDashboardComponent'))
const RSSComponent = lazy(() => import('./components/RSSComponent/RSSComponent'))

function AppContent() {
  const { getToken } = useAuthToken()
  const [tokenGetterReady, setTokenGetterReady] = useState(false)

  // Register the token getter for RTK Query before any query consumer mounts.
  // Auth0 guarantees we can mint a token, but the getter plumbing is custom;
  // gate rendering on it so RTK Query effects never run before it's set.
  useEffect(() => {
    setTokenGetter(getToken)
    setTokenGetterReady(true)
  }, [getToken])

  if (!tokenGetterReady) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="100vh">
        <CircularProgress />
      </Box>
    )
  }

  return (
    <>
      <AppBar position="sticky" color="default" elevation={0}>
        <Toolbar>
          <Typography variant="h6" component="h1" sx={{ flex: 1 }}>
            Cobblepod Dashboard
          </Typography>
          <Suspense fallback={<CircularProgress size={24} />}>
            <AccountMenu />
          </Suspense>
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
