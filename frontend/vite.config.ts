import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    // Keep the API same-origin in development. The admin session cookie is
    // HttpOnly + SameSite=Lax, so proxying `/api` avoids CORS and keeps the
    // cookie first-party. In production the SPA is served from the same origin
    // as the API and this block is unused.
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: false,
      },
    },
  },
})
