import { Outlet, ScrollRestoration } from 'react-router'
import { Toaster } from '@/components/ui/sonner'

export function RootLayout() {
  return (
    <>
      <Outlet />
      <Toaster theme="light" style={{ zIndex: 200 }} />
      <ScrollRestoration />
    </>
  )
}
