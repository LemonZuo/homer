import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// 通过 .env / .env.local 读取 dev server 代理目标，方便每人/每机器各自配置。
// 复制 .env.example 为 .env.local 后改成自己后端地址。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.API_PROXY_TARGET || 'http://localhost:8080'

  return {
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
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
