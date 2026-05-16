/// <reference types="vitest" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { execSync } from 'node:child_process'
import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf-8'))

const appVersion = process.env.VITE_APP_VERSION || pkg.version || 'dev'

let gitCommit = process.env.VITE_GIT_COMMIT || ''
if (!gitCommit) {
  try {
    gitCommit = execSync('git rev-parse --short HEAD').toString().trim()
  } catch {
    gitCommit = 'unknown'
  }
}

const buildTime = process.env.VITE_BUILD_TIME || new Date().toISOString()

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
    __GIT_COMMIT__: JSON.stringify(gitCommit),
    __BUILD_TIME__: JSON.stringify(buildTime),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws':  { target: 'ws://localhost:8080',  ws: true, changeOrigin: true },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.ts'],
    setupFiles: ['src/test/setup.ts'],
  },
})
