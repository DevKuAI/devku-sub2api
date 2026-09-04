export interface ApiKeyName {
  name?: string | null
  display_name?: string | null
}

export interface ManagedApiKey {
  managed_by?: string | null
}

export const getApiKeyDisplayName = (key?: ApiKeyName | null): string =>
  key?.display_name?.trim() || key?.name?.trim() || ''

export const isDesktopManagedApiKey = (key?: ManagedApiKey | null): boolean =>
  key?.managed_by === 'desktop'
