import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  base: './', 
  server: {
    proxy: {
      // '/api' で始まるリクエストをバックエンドサーバーに転送する
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})