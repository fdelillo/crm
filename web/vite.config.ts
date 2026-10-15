import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

export default defineConfig(({ command, isPreview }) => {
  const development = command === 'serve' && !isPreview
  const cert = new URL('../.certs/localhost.pem', import.meta.url)
  const key = new URL('../.certs/localhost-key.pem', import.meta.url)
  if (development && (!existsSync(cert) || !existsSync(key))) {
    throw new Error(
      'Faltan los certificados de desarrollo: corré `make dev-certs` (ver README).',
    )
  }
  return {
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
    server: development
      ? {
          host: 'localhost',
          port: 5173,
          strictPort: true,
          https: { cert: readFileSync(cert), key: readFileSync(key) },
          proxy: { '/api': { target: 'https://localhost:8443' } },
        }
      : undefined,
    build: {
      target: ['chrome112', 'edge112', 'firefox128', 'safari16.4', 'ios16.4'],
    },
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
  }
})
