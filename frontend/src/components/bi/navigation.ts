export function biAccessPath(organizationId: string, selfManaged = false) {
  return selfManaged ? '/desktop/organization/bi' : `/admin/desktop/organizations/${encodeURIComponent(organizationId)}/bi`
}

// Only named, local sources are carried between pages; never accept arbitrary return URLs.
export function biAccessQuery(from: unknown) {
  return { from: from === 'identities' || from === 'onboarding' ? from : 'organization' }
}
