import { webcrypto } from 'node:crypto'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const client = vi.hoisted(() => ({
  get: vi.fn(),
  request: vi.fn(),
  put: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiClient: client }))
import {
  createOrganizationContact,
  createReceivingAccount,
  deleteOrganizationContact,
  getRecord,
  listRecords,
  mutate,
  organizationContacts,
  receivingAccounts,
  stableRequest,
  updateOrganizationContact,
  updateReceivingAccount,
  upload,
} from '../api'
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
  it('loads one admin accounting record for an edit page', async () => {
    client.get.mockResolvedValueOnce({ data: { id: 'card-one' } })
    await getRecord('vcc-cards', 'card-one')
    expect(client.get).toHaveBeenCalledWith(
      '/admin/accounting/vcc-cards/card-one',
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
  it('uses the admin contact endpoints with idempotency keys', async () => {
    const input = {
      organization_id: 'org_one',
      name: '王财务',
      role: 'FINANCE' as const,
      phone: '13800138000',
      email: 'finance@example.test',
      wechat: '',
      remark: '',
      is_primary: true,
    }
    client.get.mockResolvedValueOnce({
      data: [
        { id: 'legacy-record', details: { contact_name: '旧数据' } },
        { ...input, id: 'contact-one' },
      ],
    })
    expect(await organizationContacts('org_one')).toEqual([
      { ...input, id: 'contact-one' },
    ])
    expect(client.get).toHaveBeenLastCalledWith(
      '/admin/accounting/organization-contacts',
      { params: { organization_id: 'org_one' } },
    )

    await createOrganizationContact(input, 'create-key')
    await updateOrganizationContact('contact-one', input, 'update-key')
    await deleteOrganizationContact('contact-one', 'delete-key')
    expect(client.request).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: '/admin/accounting/organization-contacts',
        method: 'post',
        headers: { 'Idempotency-Key': 'create-key' },
      }),
    )
    expect(client.request).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: '/admin/accounting/organization-contacts/contact-one',
        method: 'patch',
        headers: { 'Idempotency-Key': 'update-key' },
      }),
    )
    expect(client.request).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({
        url: '/admin/accounting/organization-contacts/contact-one',
        method: 'delete',
        headers: { 'Idempotency-Key': 'delete-key' },
      }),
    )
  })
  it('uses global receiving-account endpoints with idempotency keys', async () => {
    const input = {
      currency: 'CNY',
      account_name: '上海星海智能科技有限公司',
      bank_name: '招商银行',
      bank_account: '6225888800001234',
      bank_branch: '上海科技园支行',
      remark: '',
    }
    client.get.mockResolvedValueOnce({
      data: [
        { id: 'legacy-record', details: { receiving_account: '旧数据' } },
        { ...input, id: 'account-one' },
      ],
    })
    expect(await receivingAccounts()).toEqual([
      { ...input, id: 'account-one' },
    ])
    expect(client.get).toHaveBeenLastCalledWith(
      '/admin/accounting/receiving-accounts',
    )

    await createReceivingAccount(input, 'create-account-key')
    await updateReceivingAccount('account-one', input, 'update-account-key')
    expect(client.request).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: '/admin/accounting/receiving-accounts',
        method: 'post',
        headers: { 'Idempotency-Key': 'create-account-key' },
      }),
    )
    expect(client.request).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: '/admin/accounting/receiving-accounts/account-one',
        method: 'patch',
        headers: { 'Idempotency-Key': 'update-account-key' },
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
