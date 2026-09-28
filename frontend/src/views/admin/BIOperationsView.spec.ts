import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getOverview: vi.fn(), getRetention: vi.fn(), getCleanup: vi.fn(), updateRetention: vi.fn(),
  listBindings: vi.fn(), listChallenges: vi.fn(), listSessions: vi.fn(), revokeBinding: vi.fn(),
  listSources: vi.fn(), listCredentials: vi.fn(), issueCredential: vi.fn(), revokeCredential: vi.fn(), rotateCredential: vi.fn(),
  listImports: vi.fn(), getImport: vi.fn(), retryImport: vi.fn(), listQuality: vi.fn(),
  listReports: vi.fn(), getReport: vi.fn(), retryReport: vi.fn(), archiveReport: vi.fn(), listAudit: vi.fn(), runCleanup: vi.fn(),
}))
const appStore = vi.hoisted(() => ({ showSuccess: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { bi: api } }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))

import BIOperationsView from './BIOperationsView.vue'

const page = <T>(items: T[], total = items.length, pageNumber = 1, pageSize = 20) => ({ items, total, page: pageNumber, page_size: pageSize })
const emptyOverview = { config: { enabled: true, appid: 'test', min_client_version: '0.1.0', app_secret_configured: true, jwt_secret_configured: true, identity_secret_configured: true, report_retention_months: 24 }, counts: {}, retention: { report_months: 24, fact_months: 24, audit_days: 180, ephemeral_days: 7 } }

function mountView() {
  return mount(BIOperationsView, {
    global: {
      stubs: {
        Pagination: { props: ['page', 'pageSize', 'total'], template: '<div data-testid="pagination" />' },
      },
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.getOverview.mockResolvedValue(emptyOverview)
  api.getRetention.mockResolvedValue(emptyOverview.retention)
  api.getCleanup.mockResolvedValue({ retention: emptyOverview.retention, last_run_at: null, last_deleted: 0 })
  api.listBindings.mockResolvedValue(page([])); api.listChallenges.mockResolvedValue(page([])); api.listSessions.mockResolvedValue(page([]))
  api.listSources.mockResolvedValue(page([])); api.listCredentials.mockResolvedValue(page([])); api.listImports.mockResolvedValue(page([])); api.listQuality.mockResolvedValue({ items: [], as_of: '2026-09-28T00:00:00Z' }); api.listReports.mockResolvedValue(page([])); api.listAudit.mockResolvedValue(page([]))
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})

describe('BIOperationsView', () => {
  it('keeps backend pagination instead of truncating operations at 100 rows', async () => {
    api.listImports.mockResolvedValue(page([{ id: 'batch-1', organization_id: 'org', source_id: 'usage', status: 'failed', errors: [], attempt_count: 1 }], 101))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '数据同步')!.trigger('click')
    await flushPromises()

    expect(api.listImports).toHaveBeenLastCalledWith({ page: 1, page_size: 20 })
    expect(wrapper.get('[data-testid="pagination"]').exists()).toBe(true)
    ;(wrapper.vm as any).setPage('imports', 2)
    await flushPromises()
    expect(api.listImports).toHaveBeenLastCalledWith({ page: 2, page_size: 20 })
    wrapper.unmount()
  })

  it('allows retrying failed imports and loads complete batch details', async () => {
    const item = { id: 'batch-failed', organization_id: 'org', source_id: 'usage', status: 'failed', errors: [{ record_index: 2, code: 'INVALID', message: 'bad row' }], attempt_count: 2 }
    api.listImports.mockResolvedValue(page([item]))
    api.retryImport.mockResolvedValue({ ...item, id: 'batch-retry', status: 'queued' })
    api.getImport.mockResolvedValue({ ...item, errors: [...item.errors, { record_index: null, code: 'BATCH', message: 'batch error' }] })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '数据同步')!.trigger('click')
    await flushPromises()

    await (wrapper.vm as any).loadImportDetails(item)
    expect(api.getImport).toHaveBeenCalledWith('batch-failed')
    expect((wrapper.vm as any).selectedImport.errors).toHaveLength(2)
    await (wrapper.vm as any).retryImport(item)
    expect(api.retryImport).toHaveBeenCalledWith('batch-failed')
    wrapper.unmount()
  })
})
