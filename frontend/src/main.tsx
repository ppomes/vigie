import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'
import './i18n/i18n'
import { ThemeProvider } from './themes/ThemeContext'
import { BrandingProvider } from './themes/BrandingContext'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <BrandingProvider>
        <App />
      </BrandingProvider>
    </ThemeProvider>
  </React.StrictMode>,
)
