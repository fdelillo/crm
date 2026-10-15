import { writeFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [
    react(),
    {
      name: 'keep-dist-placeholder',
      writeBundle() {
        // The Go embed package must also compile from a clean checkout.
        writeFileSync(new URL('./dist/.gitkeep', import.meta.url), '')
      },
    },
  ],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
})
