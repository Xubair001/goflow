import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // Lands the build directly where Go's embed directive can see it --
    // embed can't reach outside its own module tree, so the build output
    // has to live inside backend/, not frontend/dist.
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
      '/openapi.yaml': 'http://localhost:8080',
      '/docs': 'http://localhost:8080',
    },
  },
})
