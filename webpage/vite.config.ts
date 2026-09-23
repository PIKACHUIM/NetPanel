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
    port: 5170,
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
    // 体积较大的第三方库单独分包：原先 echarts/xterm 会被打进首屏 chunk，
    // 产生两个 1MB+ 的入口包。拆出后首屏只需加载实际访问到的库。
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
          antd: ['antd', '@ant-design/icons'],
          echarts: ['echarts', 'echarts-for-react'],
          xterm: ['xterm', 'xterm-addon-fit', 'xterm-addon-web-links'],
          i18n: ['i18next', 'react-i18next'],
        },
      },
    },
  },
})
