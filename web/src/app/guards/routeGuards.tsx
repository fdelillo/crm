import { Outlet } from 'react-router'

export function PublicOnly() {
  return <Outlet />
}
export function RequireSession() {
  return <Outlet />
}
export function RequirePermission() {
  return <Outlet />
}
