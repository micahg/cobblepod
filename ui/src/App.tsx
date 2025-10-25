import { useState, lazy, Suspense } from 'react'
import { Provider } from 'react-redux'
import { store } from './store/store'
import reactLogo from './assets/react.svg'
import viteLogo from '/vite.svg'
import './App.css'
import { AuthGuard, UserProfile, LogoutButton } from './auth'

// Lazy load the UploadBackupComponent
const UploadBackupComponent = lazy(() => import('./components/UploadBackupComponent/UploadBackupComponent'))

function App() {
  const [count, setCount] = useState(0)

  return (
    <Provider store={store}>
      <AuthGuard>
        <div>
          <a href="https://vite.dev" target="_blank">
            <img src={viteLogo} className="logo" alt="Vite logo" />
          </a>
          <a href="https://react.dev" target="_blank">
            <img src={reactLogo} className="logo react" alt="React logo" />
          </a>
        </div>
        
        <h1>Cobblepod Dashboard</h1>
        
        <UserProfile />
        
        <div className="card">
          <button onClick={() => setCount((count) => {
            return count + 1
          })}>
            count is {count}
          </button>
          <p>
            Edit <code>src/App.tsx</code> and save to test HMR
          </p>
        </div>

        {/* Upload Backup Component */}
        <Suspense fallback={<div>Loading upload component...</div>}>
          <UploadBackupComponent />
        </Suspense>
        
        <div style={{ marginTop: '20px' }}>
          <LogoutButton />
        </div>
        
        <p className="read-the-docs">
          Click on the Vite and React logos to learn more
        </p>
      </AuthGuard>
    </Provider>
  )
}

export default App
