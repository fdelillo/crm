import { describe, expect, it } from 'vitest'
import { buildUser } from '@/test/fixtures'
import { availableUserActions, countActiveAdmins } from './rules'

describe('user action rules', () => {
  // T-F501 (BR-F01..03, INV-F12, DD-26): actions reflect state, self and verification.
  it('offers the allowed actions for each user state and protects the last active admin', () => {
    const otherAdmin = buildUser({ id: 'other', role: 'admin', status: 'active' })
    const self = buildUser({ id: 'self', role: 'admin', status: 'active' })
    expect(availableUserActions(otherAdmin, { currentUserId: 'self', activeAdminCount: 2, actorEmailVerified: true })).toEqual([
      { action: 'makeOperator', enabled: true, requiresConfirmation: true },
      { action: 'deactivate', enabled: true, requiresConfirmation: true },
    ])
    expect(availableUserActions(self, { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: true })).toEqual([
      { action: 'makeOperator', enabled: false, disabledReason: 'last_admin', requiresConfirmation: true },
      { action: 'deactivate', enabled: false, disabledReason: 'last_admin', requiresConfirmation: true },
    ])
    expect(availableUserActions(self, { currentUserId: 'self', activeAdminCount: 2, actorEmailVerified: true })).toEqual([
      { action: 'makeOperator', enabled: true, requiresConfirmation: true, selfWarning: 'lose_settings_access' },
      { action: 'deactivate', enabled: true, requiresConfirmation: true, selfWarning: 'end_own_session' },
    ])
  })

  it('keeps invited users outside the active admin count and applies verification gates', () => {
    expect(countActiveAdmins([
      buildUser({ role: 'admin', status: 'active' }),
      buildUser({ role: 'admin', status: 'invited' }),
      buildUser({ role: 'admin', status: 'disabled' }),
    ])).toBe(1)
    const invitedOperator = buildUser({ id: 'invited', role: 'operator', status: 'invited' })
    expect(availableUserActions(invitedOperator, { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: true })).toEqual([
      { action: 'resendInvitation', enabled: true, requiresConfirmation: false },
      { action: 'makeAdmin', enabled: true, requiresConfirmation: true },
      { action: 'deactivate', enabled: true, requiresConfirmation: true },
    ])
    expect(availableUserActions(invitedOperator, { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: false })).toEqual([
      { action: 'resendInvitation', enabled: false, disabledReason: 'email_not_verified', requiresConfirmation: false },
      { action: 'makeAdmin', enabled: true, requiresConfirmation: true },
      { action: 'deactivate', enabled: true, requiresConfirmation: true },
    ])
    expect(availableUserActions(buildUser({ role: 'admin', status: 'invited' }), { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: true })).toEqual([
      { action: 'makeOperator', enabled: true, requiresConfirmation: true },
      { action: 'resendInvitation', enabled: true, requiresConfirmation: false },
      { action: 'deactivate', enabled: true, requiresConfirmation: true },
    ])
  })

  it('offers role changes to active operators and only reactivation to disabled users', () => {
    expect(availableUserActions(buildUser({ role: 'operator', status: 'active' }), { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: true }).map(({ action }) => action)).toEqual(['makeAdmin', 'deactivate'])
    expect(availableUserActions(buildUser({ role: 'admin', status: 'disabled', name: null }), { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: false })).toEqual([
      { action: 'reactivate', enabled: false, disabledReason: 'email_not_verified', requiresConfirmation: true },
    ])
    expect(availableUserActions(buildUser({ role: 'admin', status: 'disabled', name: 'Invitado previo' }), { currentUserId: 'self', activeAdminCount: 1, actorEmailVerified: false })).toEqual([
      { action: 'reactivate', enabled: true, requiresConfirmation: true },
    ])
  })
})
