import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import './index.css'
import App from './App.tsx'
import { TooltipProvider } from './components/ui/tooltip'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <TooltipProvider delayDuration={200}>
        <App />
      </TooltipProvider>
      <Toaster
        position="top-center"
        toastOptions={{
          style: { fontSize: '13px' },
          className: 'font-sans',
        }}
      />
    </BrowserRouter>
  </StrictMode>,
)
