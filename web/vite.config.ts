import { writeFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: 'keep-dist-placeholder',
      writeBundle() {
        // The Go embed package must also compile from a clean checkout.
        writeFileSync(new URL('./dist/.gitkeep', import.meta.url), '')
      },
    },
  ],
  build: {
    target: ['chrome112', 'edge112', 'firefox128', 'safari16.4', 'ios16.4'],
  },
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
})
