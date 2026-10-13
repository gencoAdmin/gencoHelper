import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig(({ mode, isSsrBuild }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return { plugins: [vue()], server: { proxy: { '/api': env.VITE_API_PROXY || 'http://127.0.0.1:8080' } }, build: { rollupOptions: { output: { manualChunks: isSsrBuild ? undefined : { charts: ['echarts'], vue: ['vue', 'vue-router', 'pinia', 'vue-i18n'] } } }, chunkSizeWarningLimit: 1100 } }
})
