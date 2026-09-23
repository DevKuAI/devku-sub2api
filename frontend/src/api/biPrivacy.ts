import { apiClient } from './client'

export interface BIPrivacyNotice {
  site_url: string
  title: string
  version: string
  content_md: string
  updated_at: string
  url: string
}

export type BIPrivacyNoticeInput = Pick<BIPrivacyNotice, 'site_url' | 'title' | 'version' | 'content_md'>

export async function getBIPrivacyNotice(admin = false): Promise<BIPrivacyNotice> {
  const { data } = await apiClient.get<BIPrivacyNotice>(`${admin ? '/admin' : ''}/settings/bi-privacy`)
  return data
}

export async function saveBIPrivacyNotice(input: BIPrivacyNoticeInput): Promise<BIPrivacyNotice> {
  const { data } = await apiClient.put<BIPrivacyNotice>('/admin/settings/bi-privacy', input)
  return data
}
