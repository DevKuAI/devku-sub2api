import { ref } from 'vue'
import * as api from './api'
import type { AccountingOrganization } from './types'

// Resolves the organization an admin accounting page is scoped to.
export function useOrganization(organizationId: () => string) {
  const organization = ref<AccountingOrganization | null>(null)
  const missing = ref(false)

  async function loadOrganization(signal?: AbortSignal) {
    const id = organizationId()
    const rows = await api.organizations({ search: id }, signal)
    organization.value = rows.find((row) => row.id === id) || null
    missing.value = !organization.value
    return organization.value
  }

  return { organization, missing, loadOrganization }
}
