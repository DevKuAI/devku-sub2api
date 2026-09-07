import type { AuthResponse } from '@/types'

const IMPERSONATION_KEY = 'sub2api_impersonation'
const SESSION_KEYS = ['auth_token', 'auth_user', 'refresh_token', 'token_expires_at', 'pending_auth_session']

// Select once per page load so in-flight requests cannot write into a different session.
export const authStorage = sessionStorage.getItem(IMPERSONATION_KEY) !== null
  ? sessionStorage
  : localStorage

export function getImpersonationEmail(): string | null {
  return sessionStorage.getItem(IMPERSONATION_KEY)
}

export function startImpersonation(response: AuthResponse): void {
  if (getImpersonationEmail() !== null) {
    throw new Error('Already impersonating a user')
  }
  try {
    sessionStorage.setItem('auth_token', response.access_token)
    sessionStorage.setItem('auth_user', JSON.stringify(response.user))
    if (response.refresh_token) {
      sessionStorage.setItem('refresh_token', response.refresh_token)
    } else {
      sessionStorage.removeItem('refresh_token')
    }
    if (response.expires_in) {
      sessionStorage.setItem('token_expires_at', String(Date.now() + response.expires_in * 1000))
    } else {
      sessionStorage.removeItem('token_expires_at')
    }
    sessionStorage.removeItem('pending_auth_session')
    sessionStorage.setItem(IMPERSONATION_KEY, response.user.email)
  } catch (error) {
    clearImpersonation()
    throw error
  }
}

export function clearImpersonation(): void {
  for (const key of SESSION_KEYS) {
    sessionStorage.removeItem(key)
  }
  sessionStorage.removeItem(IMPERSONATION_KEY)
}
