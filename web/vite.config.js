import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: '/admin/',
  build: { outDir: '../server/web', emptyOutDir: true, chunkSizeWarningLimit: 3000 },
  server: { host: true, port: 5180 },
})
