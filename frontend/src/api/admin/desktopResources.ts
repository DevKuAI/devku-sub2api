import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type ResourceKind = 'mcp' | 'skill'
export type ResourceStatus = 'active' | 'disabled'
export interface ResourceManifest {
  schemaVersion: 1
  key: string
  kind: ResourceKind
  name: string
  description: string
  sourceUrl: string
  version: string
  platform: string
  entry: string
  args?: string[]
  credentialMode?: '' | 'enterprise_model'
  targets: string[]
  files: Record<string, string>
}
export interface ResourceArtifact {
  platform: string
  sha256: string
  sizeBytes: number
  manifest: ResourceManifest
  createdBy?: number
  createdAt?: string
}
export interface DesktopResource {
  id: string
  key: string
  kind: ResourceKind
  name: string
  description: string
  sourceUrl: string
  version: string
  scope: 'public' | 'enterprise'
  artifacts: ResourceArtifact[]
}
export interface ResourceRecord extends DesktopResource {
  status: ResourceStatus
  statusReason: string
  createdBy: number
  updatedBy: number
  createdAt: string
  updatedAt: string
}
export interface ResourceVersion {
  version: string
  name: string
  description: string
  sourceUrl: string
  createdBy: number
  createdAt: string
  artifacts: ResourceArtifact[]
}
export interface ResourceValidation {
  manifest: ResourceManifest
  sha256: string
  sizeBytes: number
  current: ResourceRecord | null
}
export interface ResourceQuery {
  page: number
  page_size: number
  search?: string
  kind?: ResourceKind | ''
  status?: ResourceStatus | ''
  key?: string
}
const base = '/admin/desktop/resources'
export async function listResources(params: ResourceQuery, signal?: AbortSignal) {
  return (await apiClient.get<PaginatedResponse<ResourceRecord>>(base, { params, signal })).data
}
export async function getResource(id: string) {
  return (await apiClient.get<ResourceRecord>(`${base}/${encodeURIComponent(id)}`)).data
}
export async function listResourceVersions(id: string, page = 1, pageSize = 20) {
  return (await apiClient.get<PaginatedResponse<ResourceVersion>>(`${base}/${encodeURIComponent(id)}/versions`, { params: { page, page_size: pageSize } })).data
}
export async function validateResource(file: File, onProgress: (value: number) => void) {
  return (await apiClient.post<ResourceValidation>(`${base}/validate`, file, zipOptions(onProgress))).data
}
export async function publishResource(file: File, onProgress: (value: number) => void) {
  const { data } = await apiClient.post<{ data: DesktopResource }>(base, file, zipOptions(onProgress))
  return data.data
}
function zipOptions(onProgress: (value: number) => void) {
  return {
    headers: { 'Content-Type': 'application/zip' },
    timeout: 300000,
    onUploadProgress: (event: { loaded: number; total?: number }) => {
      if (event.total) onProgress(Math.min(100, Math.round(event.loaded * 100 / event.total)))
    },
  }
}
export async function setResourceStatus(id: string, status: ResourceStatus, reason: string) {
  return (await apiClient.patch<ResourceRecord>(`${base}/${encodeURIComponent(id)}/status`, { status, reason })).data
}
export function resourceError(error: unknown): { status: number; code: string; message: string } {
  const value = error as { status?: number; code?: string; reason?: string; message?: string; error?: { code?: string; message?: string; details?: { detail?: string } }; metadata?: { detail?: string } }
  return {
    status: value?.status ?? 0,
    code: value?.error?.code || value?.reason || value?.code || '',
    message: value?.error?.details?.detail || value?.metadata?.detail || value?.error?.message || value?.message || '',
  }
}

// A timeout or a conflict alone is never evidence that our bytes were published.
export async function findPublishedResource(preview: ResourceValidation): Promise<ResourceRecord | null> {
  const resources = await listResources({ page: 1, page_size: 1, key: preview.manifest.key })
  const resource = resources.items[0]
  if (!resource) return null
  for (let page = 1; page <= 101; page++) {
    const result = await listResourceVersions(resource.id, page, 100)
    const version = result.items.find(item => item.version === preview.manifest.version)
    if (version) {
      const artifact = version.artifacts.find(item => item.platform === preview.manifest.platform)
      return artifact?.sha256 === preview.sha256 && artifact.sizeBytes === preview.sizeBytes && artifact.manifest.key === preview.manifest.key ? resource : null
    }
    if (page * 100 >= result.total) break
  }
  return null
}

export interface ResourceDownloadURL {
  url: string
  expiresAt: string
  expiresIn: number
  sha256: string
  sizeBytes: number
}
export async function getResourceDownloadURL(id: string, version: string, platform: string) {
  return (await apiClient.post<ResourceDownloadURL>(`${base}/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}/artifacts/${encodeURIComponent(platform)}/download-url`)).data
}
