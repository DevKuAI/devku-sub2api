import { beforeEach, describe, expect, it, vi } from 'vitest'

const apiClientMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/client', () => ({ apiClient: apiClientMock }))

import {
  createUpdateRelease,
  createOrganization,
  getGatewayUser,
  listAvailableGatewayUsers,
  listOrganizations,
  publishUpdateRelease,
  rotateModelToken,
  uploadUpdateArtifact,
  withdrawUpdateRelease,
} from '@/api/admin/desktop'

describe('Desktop admin API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('queries only users available for Desktop assignment', async () => {
    apiClientMock.get.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 30, pages: 0 } })

    await listAvailableGatewayUsers('carrier')

    expect(apiClientMock.get).toHaveBeenCalledWith('/admin/users', expect.objectContaining({
      params: {
        page: 1,
        page_size: 30,
        search: 'carrier',
        available_for_desktop: true,
      },
    }))
  })

  it('passes organization pagination and filters without local rewriting', async () => {
    apiClientMock.get.mockResolvedValue({ data: { items: [], total: 0, page: 2, page_size: 50, pages: 0 } })

    await listOrganizations(2, 50, { search: 'desktop', status: 'disabled' })

    expect(apiClientMock.get).toHaveBeenCalledWith('/admin/desktop/organizations', expect.objectContaining({
      params: { page: 2, page_size: 50, search: 'desktop', status: 'disabled' },
    }))
  })

  it('hydrates the currently assigned user through the normal admin endpoint', async () => {
    apiClientMock.get.mockResolvedValue({ data: { id: 42 } })

    await getGatewayUser(42)

    expect(apiClientMock.get).toHaveBeenCalledWith('/admin/users/42')
  })

  it('adds independent idempotency keys to provisioning writes', async () => {
    apiClientMock.post.mockResolvedValue({ data: {} })

    await createOrganization({ code: 'desktop', name: 'Desktop', gateway_user_id: 42, group_id: 7 })
    await rotateModelToken('org one', 'member one')

    const createHeaders = apiClientMock.post.mock.calls[0][2].headers
    const rotateHeaders = apiClientMock.post.mock.calls[1][2].headers
    expect(createHeaders['Idempotency-Key']).toMatch(/^desktop-organization-create-/)
    expect(rotateHeaders['Idempotency-Key']).toMatch(/^desktop-model-token-rotate-/)
    expect(createHeaders['Idempotency-Key']).not.toBe(rotateHeaders['Idempotency-Key'])
    expect(apiClientMock.post.mock.calls[1][0]).toContain('/org%20one/members/member%20one/')
  })

  it('adds independent idempotency keys to release lifecycle writes', async () => {
    apiClientMock.post.mockResolvedValue({ data: {} })
    const artifacts = {
      'darwin-aarch64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
      'darwin-x86_64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
      'windows-x86_64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
    }

    await createUpdateRelease({ version: '1.2.3', notes: 'Notes', artifacts })
    await publishUpdateRelease('upd one')
    await withdrawUpdateRelease('upd one', 'Rollback')

    const headers = apiClientMock.post.mock.calls.map((call) => call[2].headers['Idempotency-Key'])
    expect(headers[0]).toMatch(/^desktop-update-create-/)
    expect(headers[1]).toMatch(/^desktop-update-publish-/)
    expect(headers[2]).toMatch(/^desktop-update-withdraw-/)
    expect(new Set(headers).size).toBe(3)
    expect(apiClientMock.post.mock.calls[1][0]).toBe('/admin/desktop/updates/upd%20one/publish')
    expect(apiClientMock.post.mock.calls[2][1]).toEqual({ reason: 'Rollback' })
  })

  it('uploads updater bundles as multipart data with a long request timeout', async () => {
    apiClientMock.post.mockResolvedValue({ data: {} })
    const file = new File(['artifact'], 'DevKu.app.tar.gz', { type: 'application/gzip' })

    await uploadUpdateArtifact('upd one', 'darwin-aarch64', file, vi.fn())

    const [url, form, config] = apiClientMock.post.mock.calls[0]
    expect(url).toBe('/admin/desktop/updates/upd%20one/artifacts/darwin-aarch64')
    expect(form).toBeInstanceOf(FormData)
    expect((form as FormData).get('file')).toBe(file)
    expect(config.timeout).toBe(10 * 60 * 1000)
    expect(config.onUploadProgress).toEqual(expect.any(Function))
  })
})
