// Remembers, per browser session, that an organization's books were opened from its enterprise page,
// so the workspace keeps offering the way back after visiting orders, records and forms.
const prefix = 'accounting-entry:'

export function rememberEnterpriseEntry(organizationId: string) {
  sessionStorage.setItem(prefix + organizationId, 'enterprise')
}

export function cameFromEnterprise(organizationId: string) {
  return sessionStorage.getItem(prefix + organizationId) === 'enterprise'
}

// Opening the accounting directory starts a fresh path, so earlier enterprise entries no longer apply.
export function forgetEnterpriseEntries() {
  for (const key of Object.keys(sessionStorage)) if (key.startsWith(prefix)) sessionStorage.removeItem(key)
}
