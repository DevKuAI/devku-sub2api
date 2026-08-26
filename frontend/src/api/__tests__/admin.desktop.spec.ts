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
  createOrganization,
  getGatewayUser,
  listAvailableGatewayUsers,
  listOrganizations,
  rotateModelToken,
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
})
