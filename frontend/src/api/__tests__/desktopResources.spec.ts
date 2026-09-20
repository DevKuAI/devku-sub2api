import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), patch: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))
import { findPublishedResource, publishResource, resourceError, validateResource, setResourceStatus, getResourceDownloadURL } from '../admin/desktopResources'
import type { ResourceValidation } from '../admin/desktopResources'

const preview = { manifest: { key: 'test', version: '1.0.0', platform: 'any' }, sha256: 'hash', sizeBytes: 10 } as ResourceValidation

describe('Desktop resource API', () => {
  beforeEach(() => { vi.resetAllMocks() })
  it('sends raw ZIP and unwraps the publication envelope separately from validation', async () => {
    const file = new File(['zip'], 'test.zip'), progress = vi.fn()
    client.post.mockResolvedValueOnce({ data: preview }).mockResolvedValueOnce({ data: { data: { id: 'res_one' } } })
    expect(await validateResource(file, progress)).toEqual(preview)
    expect(await publishResource(file, progress)).toEqual({ id: 'res_one' })
    expect(client.post).toHaveBeenNthCalledWith(2, '/admin/desktop/resources', file, expect.objectContaining({ headers: { 'Content-Type': 'application/zip' }, timeout: 300000 }))
    client.post.mock.calls[0][2].onUploadProgress({ loaded: 5, total: 10 })
    expect(progress).toHaveBeenCalledWith(50)
  })
  it.each(['hash', 'different'])('only confirms the exact immutable artifact hash: %s', async (hash) => {
    client.get.mockResolvedValueOnce({ data: { items: [{ id: 'res_one' }] } })
      .mockResolvedValueOnce({ data: { total: 101, items: [{ version: '2.0.0' }] } })
      .mockResolvedValueOnce({ data: { total: 101, items: [{ version: '1.0.0', artifacts: [{ platform: 'any', sha256: hash, sizeBytes: 10, manifest: { key: 'test' } }] }] } })
    expect(await findPublishedResource(preview)).toEqual(hash === 'hash' ? { id: 'res_one' } : null)
    expect(client.get.mock.calls[0][1].params).toEqual({ page: 1, page_size: 1, key: 'test' })
    expect(client.get.mock.calls[2][1].params.page).toBe(2)
  })
  it('normalizes both error envelopes and proxy failures', () => {
    expect(resourceError({ status: 400, error: { code: 'RESOURCE_PACKAGE_INVALID', message: 'invalid', details: { detail: 'bad hash' } } })).toEqual({ status: 400, code: 'RESOURCE_PACKAGE_INVALID', message: 'bad hash' })
    expect(resourceError({ status: 423, code: 'ADMIN_COMPLIANCE_ACK_REQUIRED', message: 'acknowledge' }).code).toBe('ADMIN_COMPLIANCE_ACK_REQUIRED')
    expect(resourceError({ status: 502, message: 'Bad Gateway' }).message).toBe('Bad Gateway')
  })
  it('escapes resource IDs and submits the disable reason', async () => {
    client.patch.mockResolvedValue({ data: {} })
    await setResourceStatus('res/one', 'disabled', 'review')
    expect(client.patch).toHaveBeenCalledWith('/admin/desktop/resources/res%2Fone/status', { status: 'disabled', reason: 'review' })
  })
  it('requests a fresh download URL for the selected immutable artifact', async () => {
    const link = { url: 'https://private.example.com/file?signature=one', expiresAt: '2026-09-20T00:05:00Z', expiresIn: 300, sha256: 'hash', sizeBytes: 10 }
    client.post.mockResolvedValue({ data: link })
    expect(await getResourceDownloadURL('res/one', '1.0.0', 'darwin-arm64')).toEqual(link)
    expect(client.post).toHaveBeenCalledWith('/admin/desktop/resources/res%2Fone/versions/1.0.0/artifacts/darwin-arm64/download-url')
  })
})
