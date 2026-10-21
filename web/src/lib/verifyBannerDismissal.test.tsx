import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearVerifyBannerDismissed,
  useVerifyBannerDismissed,
} from './verifyBannerDismissal'

const key = 'crm.verifyBanner.dismissed'

describe('email verification notice dismissal', () => {
  beforeEach(() => sessionStorage.clear())

  it('stores a fixed boolean mark and restores it after remount', () => {
    const first = renderHook(() => useVerifyBannerDismissed())
    expect(first.result.current[0]).toBe(false)

    act(() => first.result.current[1]())

    expect(sessionStorage.getItem(key)).toBe('true')
    expect(
      [...Array(sessionStorage.length)].map((_, index) =>
        sessionStorage.key(index),
      ),
    ).toEqual([key])
    expect(first.result.current[0]).toBe(true)

    first.unmount()
    const remounted = renderHook(() => useVerifyBannerDismissed())
    expect(remounted.result.current[0]).toBe(true)
  })

  it('clears the mark when the session changes', () => {
    sessionStorage.setItem(key, 'true')

    clearVerifyBannerDismissed()

    expect(sessionStorage.getItem(key)).toBeNull()
  })
})
