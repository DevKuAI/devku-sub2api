import { defineConfig } from 'vitest/config'
import { resolve } from 'node:path'

const root = resolve(__dirname, '../..')

export default defineConfig({
  root,
  resolve: {
    alias: {
      '@': resolve(root, 'src'),
      'axios': resolve(root, 'node_modules/axios/dist/node/axios.cjs'),
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js',
    },
  },
  test: {
    environment: 'jsdom',
    include: ['tests/desktop-structure/client.http.ts'],
    minWorkers: 1,
    maxWorkers: 1,
    testTimeout: 30000,
  },
})
