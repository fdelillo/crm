import { StrictMode, lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/globals.css'
import { Application, createApplication } from './app/application'

const QueryDevtools = import.meta.env.DEV
  ? lazy(() =>
      import('@tanstack/react-query-devtools').then((module) => ({
        default: module.ReactQueryDevtools,
      })),
    )
  : () => null
const application = createApplication()
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Application {...application}>
      <Suspense>
        <QueryDevtools />
      </Suspense>
    </Application>
  </StrictMode>,
)

// ADR-020: the worker caches only the standalone offline page, never app or API data.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  const register = () => {
    void navigator.serviceWorker
      .register('/sw.js', { scope: '/' })
      .catch(() => undefined)
  }
  if (document.readyState === 'complete') register()
  else window.addEventListener('load', register, { once: true })
}
