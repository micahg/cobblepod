import { lazy, Suspense, useEffect } from 'react'
import { Provider } from 'react-redux'
import { store } from './store/store'
import './App.css'
import { AuthGuard, LogoutButton, useAuthToken } from './auth'
import { setTokenGetter } from './services/backupApi'

// Lazy load the UploadBackupComponent
const UploadBackupComponent = lazy(() => import('./components/UploadBackupComponent/UploadBackupComponent'))

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
      <h1>Cobblepod Dashboard</h1>

      {/* Upload Backup Component */}
      <Suspense fallback={<div>Loading upload component...</div>}>
        <UploadBackupComponent />
      </Suspense>
      
      <div style={{ marginTop: '20px' }}>
        <LogoutButton />
      </div>
    </>
  )
}

function App() {
  return (
    <Provider store={store}>
      <AuthGuard>
        <AppContent />
      </AuthGuard>
    </Provider>
  )
}

export default App
