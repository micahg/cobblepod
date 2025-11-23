import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { Auth0Provider } from './auth'
import { loadRuntimeConfig } from './config/runtime'

// Load runtime configuration before rendering the app
loadRuntimeConfig()
  .then(() => {
    createRoot(document.getElementById('root')!).render(
      <StrictMode>
        <Auth0Provider>
          <App />
        </Auth0Provider>
      </StrictMode>,
    )
  })
  .catch((error) => {
    console.error('Failed to initialize app:', error);
    document.getElementById('root')!.innerHTML = 
      '<div style="padding: 20px; color: red;">Failed to load application configuration. Please check the console for details.</div>';
  });

