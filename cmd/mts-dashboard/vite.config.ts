import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: resolve(__dirname, '../mts-server/dashboard-dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8086',
      '/healthz': 'http://127.0.0.1:8086',
      '/readyz': 'http://127.0.0.1:8086',
      '/metrics': 'http://127.0.0.1:8086',
    },
  },
})
