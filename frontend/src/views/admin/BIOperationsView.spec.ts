import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getOverview: vi.fn(), getRetention: vi.fn(), getCleanup: vi.fn(), updateRetention: vi.fn(),
  listBindings: vi.fn(), listChallenges: vi.fn(), listSessions: vi.fn(), revokeBinding: vi.fn(),
  listSources: vi.fn(), listCredentials: vi.fn(), issueCredential: vi.fn(), revokeCredential: vi.fn(), rotateCredential: vi.fn(), syncSub2api: vi.fn(),
  listImports: vi.fn(), getImport: vi.fn(), retryImport: vi.fn(), listQuality: vi.fn(),
  listReports: vi.fn(), getReport: vi.fn(), retryReport: vi.fn(), archiveReport: vi.fn(), listAudit: vi.fn(), runCleanup: vi.fn(),
}))
const appStore = vi.hoisted(() => ({ showSuccess: vi.fn() }))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))

vi.mock('@/api/admin', () => ({ adminAPI: { bi: api } }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('vue-router', () => ({ useRoute: () => route }))

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
  route.query = {}
  api.getOverview.mockResolvedValue(emptyOverview)
  api.getRetention.mockResolvedValue(emptyOverview.retention)
  api.getCleanup.mockResolvedValue({ retention: emptyOverview.retention, last_run_at: null, last_deleted: 0 })
  api.listBindings.mockResolvedValue(page([])); api.listChallenges.mockResolvedValue(page([])); api.listSessions.mockResolvedValue(page([]))
  api.listSources.mockResolvedValue(page([])); api.listCredentials.mockResolvedValue(page([])); api.listImports.mockResolvedValue(page([])); api.listQuality.mockResolvedValue({ items: [], as_of: '2026-09-28T00:00:00Z' }); api.listReports.mockResolvedValue(page([])); api.listAudit.mockResolvedValue(page([]))
  api.syncSub2api.mockResolvedValue({ status: 'idle', records: 0, job: null })
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})

describe('BIOperationsView', () => {
  const source = { organization_id: 'org_one', source_id: 'source_sub2api', namespace: 'sub2api', allowed_kinds: ['usage'], checkpoint: null, credential_count: 1, complete_through: null, initial_backfill_complete: false }
  const job = { id: 'batch-new', organization_id: 'org_one', source_id: 'source_sub2api', status: 'queued', errors: [], attempt_count: 0 }

  it('submits the selected enterprise source, blocks double submission and tracks processing to applied', async () => {
    route.query = { tab: 'sources', organization_id: 'org_one' }
    api.listSources.mockResolvedValue(page([source]))
    let resolveSync: (value: unknown) => void = () => {}
    api.syncSub2api.mockImplementationOnce(() => new Promise(resolve => { resolveSync = resolve }))
    api.getImport.mockResolvedValueOnce({ ...job, status: 'processing' }).mockResolvedValueOnce({ ...job, status: 'applied' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '开始同步 sub2api 用量')!.trigger('click')
    const form = wrapper.get('[data-testid="source-sync"] form')
    await form.get('input[type="password"]').setValue('bic_private_test')
    await form.get('input[type="datetime-local"]').setValue('2026-01-01T00:00')
    await form.trigger('submit')
    await form.trigger('submit')
    expect(api.syncSub2api).toHaveBeenCalledTimes(1)
    expect(api.syncSub2api).toHaveBeenCalledWith('source_sub2api', { organization_id: 'org_one', connector_token: 'bic_private_test', since: new Date('2026-01-01T00:00').toISOString(), limit: 500 })
    resolveSync({ status: 'queued', records: 500, job: { id: 'batch-new' } })
    await flushPromises()
    expect(wrapper.get('[data-testid="source-sync"]').text()).toContain('processing')
    expect(form.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(button => button.text() === '刷新结果')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('本批次已导入')
    expect(form.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    await form.trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('该时间范围没有可导入日志')
    expect(api.syncSub2api).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('retains a submitted batch when status lookup fails and shows rejected records without allowing a new batch', async () => {
    route.query = { tab: 'sources' }
    api.listSources.mockResolvedValue(page([source]))
    api.syncSub2api.mockResolvedValueOnce({ status: 'queued', records: 1, job: { id: job.id } })
    api.getImport.mockRejectedValueOnce(new Error('状态暂不可用')).mockResolvedValueOnce({ ...job, status: 'rejected', errors: [{ message: 'invalid source record' }] })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '开始同步 sub2api 用量')!.trigger('click')
    const panel = wrapper.get('[data-testid="source-sync"]')
    await panel.get('input[type="password"]').setValue('bic_test')
    await panel.get('form').trigger('submit')
    await flushPromises()
    expect(panel.text()).toContain('状态暂不可用')
    expect(panel.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await panel.findAll('button').find(button => button.text() === '刷新结果')!.trigger('click')
    await flushPromises()
    expect(panel.text()).toContain('invalid source record')
    expect(panel.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('uses the newly issued credential only for its own source and excludes sources that declare completeness', async () => {
    route.query = { tab: 'sources', organization_id: 'org_one' }
    api.listSources.mockResolvedValue(page([source, { ...source, source_id: 'source_other' }, { ...source, source_id: 'authoritative', complete_through: '2026-01-01T00:00:00Z' }]))
    api.issueCredential.mockResolvedValueOnce({ credential_id: 'credential', token: 'bic_once' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '填入 sub2api 推荐值')!.trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const buttons = wrapper.findAll('button').filter(button => button.text() === '开始同步 sub2api 用量')
    expect(buttons).toHaveLength(2)
    await buttons[0].trigger('click')
    expect((wrapper.get('[data-testid="source-sync"] input[type="password"]').element as HTMLInputElement).value).toBe('bic_once')
    await buttons[1].trigger('click')
    expect((wrapper.get('[data-testid="source-sync"] input[type="password"]').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })

  it('explains expired credentials and permits correcting the credential before retrying', async () => {
    route.query = { tab: 'sources' }
    api.listSources.mockResolvedValue(page([source]))
    api.syncSub2api.mockRejectedValueOnce({ code: 400, reason: 'INVALID_CONNECTOR_CREDENTIAL', message: 'Invalid credential' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '开始同步 sub2api 用量')!.trigger('click')
    const panel = wrapper.get('[data-testid="source-sync"]')
    await panel.get('input[type="password"]').setValue('bic_expired')
    await panel.get('form').trigger('submit')
    await flushPromises()
    expect(panel.get('[role="alert"]').text()).toContain('凭证无效、已过期或已撤销')
    expect(panel.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    await panel.get('input[type="password"]').setValue('bic_new')
    await panel.get('form').trigger('submit')
    await flushPromises()
    expect(panel.text()).toContain('该时间范围没有可导入日志')
    wrapper.unmount()
  })

  it('opens the organization source setup and keeps the one-time token visible after a refresh failure', async () => {
    route.query = { tab: 'sources', organization_id: 'org_one' }
    let resolveCredential: (result: { credential_id: string; token: string }) => void = () => {}
    api.issueCredential.mockImplementation(() => new Promise(resolve => { resolveCredential = resolve }))
    api.listSources.mockResolvedValueOnce(page([])).mockRejectedValueOnce(new Error('列表暂不可用'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('当前企业：org_one')
    expect(wrapper.text()).toContain('该企业尚无数据源')
    expect(api.listSources).toHaveBeenCalledWith({ page: 1, page_size: 20, organization_id: 'org_one' })
    const organizationInput = wrapper.findAll('label').find(label => label.text().includes('企业 ID'))!.find('input')
    expect((organizationInput.element as HTMLInputElement).value).toBe('org_one')
    expect(organizationInput.attributes('readonly')).toBeDefined()
    await wrapper.findAll('button').find(button => button.text() === '填入 sub2api 推荐值')!.trigger('click')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.find('button[aria-busy="true"]').exists()).toBe(true)
    await wrapper.find('form').trigger('submit')
    expect(api.issueCredential).toHaveBeenCalledTimes(1)
    resolveCredential({ credential_id: 'credential_one', token: 'bic_once' })
    await flushPromises()

    expect(api.issueCredential).toHaveBeenCalledWith(expect.objectContaining({
      organization_id: 'org_one', source_id: 'source_sub2api', namespace: 'sub2api', allowed_kinds: ['usage'],
    }))
    const issuedExpiry = new Date(api.issueCredential.mock.calls[0][0].expires_at).getTime()
    expect(issuedExpiry).toBeGreaterThan(Date.now() + 23 * 60 * 60 * 1000)
    expect(issuedExpiry).toBeLessThan(Date.now() + 25 * 60 * 60 * 1000)
    expect(wrapper.text()).toContain('bic_once')
    expect(wrapper.get('[role="alert"]').text()).toContain('列表暂不可用')
    wrapper.unmount()
  })

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
