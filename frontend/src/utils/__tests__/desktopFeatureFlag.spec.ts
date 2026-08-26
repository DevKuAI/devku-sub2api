import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as null | { desktop_enabled?: boolean },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

describe('Desktop feature flag', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
  })

  it('is opt-in and hidden until explicitly enabled', () => {
    expect(FeatureFlags.desktop.mode).toBe('opt-in')
    expect(isFeatureFlagEnabled(FeatureFlags.desktop)).toBe(false)

    appStore.cachedPublicSettings = { desktop_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.desktop)).toBe(false)

    appStore.cachedPublicSettings = { desktop_enabled: true }
    expect(isFeatureFlagEnabled(FeatureFlags.desktop)).toBe(true)
  })
})
