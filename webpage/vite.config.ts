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
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // vendor 拆分：antd/react/echarts 独立 chunk，业务代码更新时利用浏览器缓存
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-antd': ['antd', '@ant-design/icons'],
        },
      },
    },
  },
})
