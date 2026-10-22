import { createBrowserRouter, type RouteObject } from 'react-router'
import { ErrorBoundary } from './ErrorBoundary'
import { RootLayout } from './RootLayout'
import {
  PublicOnly,
  RequirePermission,
  RequireSession,
} from './guards/routeGuards'

type AuthPage = keyof typeof import('@/features/auth/pages')

const authPage = (page: AuthPage) => async () => {
  switch (page) {
    case 'LoginPage':
      return {
        Component: (await import('@/features/auth/LoginPage')).LoginPage,
      }
    case 'SignupPage':
      return {
        Component: (await import('@/features/auth/SignupPage')).SignupPage,
      }
    case 'ForgotPasswordPage':
      return {
        Component: (await import('@/features/auth/ForgotPasswordPage'))
          .ForgotPasswordPage,
      }
    case 'ResetPasswordPage':
      return {
        Component: (await import('@/features/auth/ResetPasswordPage'))
          .ResetPasswordPage,
      }
    case 'VerifyEmailPage':
      return {
        Component: (await import('@/features/auth/VerifyEmailPage'))
          .VerifyEmailPage,
      }
    case 'AcceptInvitationPage':
      return {
        Component: (await import('@/features/auth/AcceptInvitationPage'))
          .AcceptInvitationPage,
      }
  }
}
const usersPage =
  (page: keyof typeof import('@/features/users/pages')) => async () => ({
    Component: (await import('@/features/users/pages'))[page],
  })

export const appRoutes: RouteObject[] = [
  {
    Component: RootLayout,
    ErrorBoundary,
    HydrateFallback: () => null,
    children: [
      {
        Component: PublicOnly,
        children: [
          { path: 'login', lazy: authPage('LoginPage') },
          { path: 'signup', lazy: authPage('SignupPage') },
          { path: 'forgot-password', lazy: authPage('ForgotPasswordPage') },
        ],
      },
      { path: 'reset-password', lazy: authPage('ResetPasswordPage') },
      { path: 'verify-email', lazy: authPage('VerifyEmailPage') },
      { path: 'accept-invitation', lazy: authPage('AcceptInvitationPage') },
      {
        Component: RequireSession,
        children: [
          {
            // P-F18: the shell (and Radix DropdownMenu) stays out of the entry chunk.
            lazy: async () => ({
              Component: (await import('./AppShell')).AppShell,
            }),
            children: [
              {
                index: true,
                lazy: async () => ({
                  Component: (
                    await import('@/features/dashboard/DashboardPage')
                  ).DashboardPage,
                }),
              },
              {
                path: 'settings',
                lazy: async () => ({
                  Component: (await import('./SettingsPage')).SettingsPage,
                }),
              },
              {
                Component: RequirePermission,
                children: [
                  {
                    path: 'settings/company',
                    lazy: async () => ({
                      Component: (await import('@/features/tenant/CompanyPage'))
                        .CompanyPage,
                    }),
                  },
                  { path: 'settings/users', lazy: usersPage('UsersPage') },
                  {
                    path: 'settings/users/invite',
                    lazy: usersPage('InviteUserPage'),
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        path: '*',
        lazy: async () => ({
          Component: (await import('./NotFoundPage')).NotFoundPage,
        }),
      },
    ],
  },
]

export function createAppRouter() {
  return createBrowserRouter(appRoutes)
}
