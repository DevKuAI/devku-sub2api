export interface ApiKeyName {
  name?: string | null
  display_name?: string | null
}

export const getApiKeyDisplayName = (key?: ApiKeyName | null): string =>
  key?.display_name?.trim() || key?.name?.trim() || ''
