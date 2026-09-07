import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthResponse } from '@/types'

const targetSession = {
  access_token: 'target-access',
  refresh_token: 'target-refresh',
  expires_in: 3600,
  user: { id: 2, email: 'target@example.com', role: 'user' }
} as AuthResponse

describe('impersonation session storage', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
    sessionStorage.clear()
    localStorage.setItem('auth_token', 'admin-access')
    localStorage.setItem('refresh_token', 'admin-refresh')
    localStorage.setItem('auth_user', JSON.stringify({ id: 1, role: 'admin' }))
  })

  afterEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
  })

  it('switches only this tab after reload and restores the original session', async () => {
    const original = await import('../authStorage')
    original.startImpersonation(targetSession)
    expect(original.authStorage).toBe(localStorage)
    expect(localStorage.getItem('auth_token')).toBe('admin-access')
    expect(localStorage.getItem('refresh_token')).toBe('admin-refresh')

    vi.resetModules()
    const switched = await import('../authStorage')
    expect(switched.authStorage).toBe(sessionStorage)
    expect(switched.authStorage.getItem('auth_token')).toBe('target-access')
    expect(switched.getImpersonationEmail()).toBe('target@example.com')
    expect(() => switched.startImpersonation(targetSession)).toThrow('Already impersonating')

    // Logout or expiry must not silently expose the administrator session.
    switched.authStorage.removeItem('auth_token')
    expect(switched.authStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('auth_token')).toBe('admin-access')
    switched.clearImpersonation()
    vi.resetModules()
    const restored = await import('../authStorage')
    expect(restored.authStorage.getItem('auth_token')).toBe('admin-access')
    expect(restored.getImpersonationEmail()).toBeNull()
  })

  it('cleans partial writes when session storage is unavailable', async () => {
    const storage = await import('../authStorage')
    const originalSetItem = Storage.prototype.setItem
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, key, value) {
      if (this === sessionStorage && key === 'refresh_token') throw new Error('quota exceeded')
      originalSetItem.call(this, key, value)
    })
    expect(() => storage.startImpersonation(targetSession)).toThrow('quota exceeded')
    expect(sessionStorage.getItem('auth_token')).toBeNull()
    expect(storage.getImpersonationEmail()).toBeNull()
    expect(localStorage.getItem('auth_token')).toBe('admin-access')
  })
})
