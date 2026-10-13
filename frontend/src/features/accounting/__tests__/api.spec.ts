import { webcrypto } from 'node:crypto'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const client = vi.hoisted(() => ({
  get: vi.fn(),
  request: vi.fn(),
  put: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiClient: client }))
import { listRecords, stableRequest, mutate, upload } from '../api'
describe('accounting request isolation and idempotency', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('crypto', webcrypto)
    client.get.mockResolvedValue({ data: { items: [], total: 0 } })
    client.request.mockResolvedValue({ data: {} })
  })
  it('keeps customer traffic on the read-only API including card history', async () => {
    await listRecords('vcc-transactions', true, { page: 1 })
    expect(client.get).toHaveBeenCalledWith(
      '/organization/accounting/vcc-card/transactions',
      expect.objectContaining({ params: { page: 1 } }),
    )
    await listRecords('transfers', false, { organization_id: 'org_one' })
    expect(client.get).toHaveBeenLastCalledWith(
      '/admin/accounting/transfers',
      expect.objectContaining({ params: { organization_id: 'org_one' } }),
    )
  })
  it('reuses a key for a failed request retry and changes it after edits', () => {
    const getKey = stableRequest()
    const first = getKey('transfers', { amount: '100' })
    expect(getKey('transfers', { amount: '100' })).toBe(first)
    expect(getKey('transfers', { amount: '101' })).not.toBe(first)
    expect(getKey('expenses', { amount: '100' })).not.toBe(first)
    const successful = getKey('expenses', { amount: '100' })
    getKey.reset()
    expect(getKey('expenses', { amount: '100' })).not.toBe(successful)
  })
  it('sends the chosen key on shared-record mutations', async () => {
    await mutate(
      'contracts/one/sharing',
      { customer_visible: true },
      'stable-key',
      'patch',
    )
    expect(client.request).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'patch',
        headers: { 'Idempotency-Key': 'stable-key' },
      }),
    )
  })
})

describe('accounting attachment upload recovery', () => {
  const bytes = new TextEncoder().encode('%PDF-1.7\nproof\n%%EOF')
  const file = {
    name: 'invoice.pdf',
    type: 'application/pdf',
    size: bytes.length,
    arrayBuffer: async () => bytes.buffer,
  } as File
  const record = {
    id: 'record-one',
    organization_id: 'org_one',
  } as import('../types').AccountingRecord
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('crypto', webcrypto)
    client.put.mockResolvedValue({})
  })
  it('renews an expired upload intent', async () => {
    client.request
      .mockResolvedValueOnce({
        data: {
          attachment: { id: 'expired', expires_at: '2000-01-01T00:00:00Z' },
        },
      })
      .mockResolvedValueOnce({
        data: {
          attachment: { id: 'renewed', expires_at: '2099-01-01T00:00:00Z' },
        },
      })
      .mockResolvedValueOnce({ data: {} })
    await upload(file, 'invoices', record)
    expect(client.put).toHaveBeenCalledWith(
      '/admin/accounting/attachments/renewed/content',
      file,
      expect.anything(),
    )
    expect(client.request).toHaveBeenCalledTimes(3)
  })
  it('creates a new intent after a previously uploaded attachment was soft deleted', async () => {
    client.request
      .mockResolvedValueOnce({
        data: {
          attachment: { id: 'deleted', expires_at: '2099-01-01T00:00:00Z' },
        },
      })
      .mockResolvedValueOnce({
        data: {
          attachment: { id: 'replacement', expires_at: '2099-01-01T00:00:00Z' },
        },
      })
      .mockResolvedValueOnce({ data: {} })
    client.put.mockRejectedValueOnce({ status: 404 }).mockResolvedValueOnce({})
    await upload(file, 'invoices', record)
    expect(client.put).toHaveBeenCalledTimes(2)
    expect(client.put).toHaveBeenLastCalledWith(
      '/admin/accounting/attachments/replacement/content',
      file,
      expect.anything(),
    )
  })
})
