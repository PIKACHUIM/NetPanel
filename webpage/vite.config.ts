import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 1087,
    proxy: {
      '/api': {
        target: 'http://localhost:1086',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../backend/embed/dist',
    emptyOutDir: true,
  },
})
