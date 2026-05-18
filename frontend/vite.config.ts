import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'react-vendor',
              test: /node_modules[\\/](react|react-dom|scheduler|react-router-dom)[\\/]/,
              priority: 30,
            },
            {
              name: 'codemirror',
              test: /node_modules[\\/](@codemirror|@uiw[\\/]react-codemirror)[\\/]/,
              priority: 25,
            },
            {
              name: 'ui-vendor',
              test: /node_modules[\\/](@radix-ui|vaul|cmdk|lucide-react|sonner|motion)[\\/]/,
              priority: 20,
              maxSize: 300 * 1024,
            },
            {
              name: 'vendor',
              test: /node_modules[\\/]/,
              priority: 10,
              maxSize: 300 * 1024,
            },
          ],
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
})
