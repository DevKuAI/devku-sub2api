import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import zh from '@/i18n/locales/zh'
import DesktopDailyReports from '../DesktopDailyReports.vue'

const api = vi.hoisted(() => ({ getDesktopMemberReports: vi.fn(), getDesktopMemberReport: vi.fn(), getDesktopSummaryReport: vi.fn(), runDesktopReports: vi.fn() }))
vi.mock('@/api/desktopReports', () => api)
vi.mock('@/api/desktopConversations', () => ({ getDesktopConversation: vi.fn() }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({
  t: (key: string) => {
    let value: unknown = zh
    for (const part of key.split('.')) value = value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined
    return typeof value === 'string' ? value : key
  },
  te: () => true,
}) }))
const props = { organizationId: 'org_one', selfManaged: true, enabled: true, analysisModel: 'test-model' }
const base = { date: '2026-10-09', timezone: 'Asia/Shanghai', analysis_model: 'test-model', status: 'completed', reason: '', expected_members: 1, completed_members: 1, missing_members: [] }
const report = { status: 'completed', content: '个人工作产出', model: 'test-model', generated_at: '2026-10-10T02:00:00Z', error: '' }
const member = { ...report, member_id: 'mem_one', name: '成员一', deleted: false, record_count: 1, source_ids: ['record-one'] }
const empty = { ...report, status: 'no_records', content: '', model: '', generated_at: null, member_id: 'mem_empty', name: '成员二', deleted: false, record_count: 0, source_ids: [] }
function render(overrides = {}) {
  return mount(DesktopDailyReports, { props: { ...props, ...overrides }, global: { stubs: { DesktopConversationViewer: true, BaseDialog: true } } })
}
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-10T04:00:00Z')); vi.clearAllMocks()
  api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [member, empty] })
  api.getDesktopMemberReport.mockResolvedValue(member)
  api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, content: '企业整体进展' } })
  api.runDesktopReports.mockResolvedValue({ status: 'pending' })
})
afterEach(() => { vi.useRealTimers() })

describe('DesktopDailyReports', () => {
  it('loads member text only after selecting a report', async () => {
    api.getDesktopMemberReports.mockResolvedValue({ ...base, total_members: 21, members: [{ ...member, content: '', source_ids: [], executions: [] }, empty] })
    const wrapper = render(); await flushPromises()
    expect(api.getDesktopMemberReport).not.toHaveBeenCalled()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    expect(wrapper.text()).not.toContain('个人工作产出')
    await wrapper.find('tbody button').trigger('click'); await flushPromises()
    expect(api.getDesktopMemberReport).toHaveBeenCalledWith('org_one', true, '2026-10-09', 'mem_one', expect.any(AbortSignal), 1)
    expect(wrapper.text()).toContain('个人工作产出')
    wrapper.unmount()
  })
  it('separates summary and member reports, retains empty members and the selected date', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('企业整体进展')
    expect(wrapper.text()).not.toContain('个人工作产出')
    expect(api.getDesktopSummaryReport).toHaveBeenCalledWith('org_one', true, '2026-10-09', expect.any(AbortSignal))
    await wrapper.get('#daily-report-tab-members').trigger('click')
    expect(wrapper.text()).toContain('成员二'); expect(wrapper.text()).toContain('当日无对话记录')
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    await wrapper.find('tbody button').trigger('click'); expect(wrapper.text()).toContain('个人工作产出')
    expect(wrapper.text()).not.toContain('企业整体进展')
    await wrapper.get('input[type="date"]').setValue('2026-10-08'); await flushPromises()
    await wrapper.get('#daily-report-tab-summary').trigger('click')
    expect((wrapper.get('input[type="date"]').element as HTMLInputElement).value).toBe('2026-10-08')
    wrapper.unmount()
  })
  it('shows each stage model history independently from current configuration', async () => {
    const execution = { id: 1, revision: 1, kind: 'summary', model: 'summary-old-model', round: 0, chunk: 1, attempt: 1, request_id: 'request-one', status: 'completed', error: '', started_at: '2026-10-10T02:00:00Z', finished_at: '2026-10-10T02:00:10Z' }
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, executions: [execution] } })
    api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [{ ...member, executions: [{ ...execution, id: 2, kind: 'member', member_id: 'mem_one', model: 'member-old-model' }] }, empty] })
    const wrapper = render({ analysisModel: 'new-configuration-model' }); await flushPromises()
    expect(wrapper.get('[data-testid="report-execution-history"]').text()).toContain('summary-old-model')
    expect(wrapper.text()).not.toContain('member-old-model')
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.find('tbody button').trigger('click')
    expect(wrapper.get('[data-testid="report-execution-history"]').text()).toContain('member-old-model')
    expect(wrapper.text()).not.toContain('summary-old-model')
    wrapper.unmount()
  })
  it('shows no-record day without allowing generation', async () => {
    api.getDesktopMemberReports.mockResolvedValue({ ...base, status: 'no_records', reason: 'no_records', expected_members: 0, completed_members: 0, members: [empty] })
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'no_records', reason: 'no_records', expected_members: 0, completed_members: 0, summary: { ...report, status: 'no_records', content: '' } })
    const wrapper = render({ selfManaged: false }); await flushPromises()
    expect(wrapper.text()).toContain('当日无对话记录，未生成总结')
    const generate = wrapper.findAll('button').find(button => button.text() === '按日期补跑')!
    expect(generate.attributes('disabled')).toBeDefined(); expect(api.runDesktopReports).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows model unavailability while keeping historical content', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, reason: 'model_unavailable', summary: { ...report, content: '历史总结' } })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('分析模型暂不可用'); expect(wrapper.text()).toContain('历史总结')
    expect(wrapper.text()).not.toContain('按日期补跑')
    wrapper.unmount()
  })
  it('requires regeneration after analysis configuration changes', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, reason: 'configuration_changed', summary: report })
    const wrapper = render({ selfManaged: false }); await flushPromises()
    const buttons = wrapper.findAll('button')
    expect(buttons.find(button => button.text() === '按日期补跑')!.attributes('disabled')).toBeDefined()
    expect(buttons.find(button => button.text() === '失败重试')!.attributes('disabled')).toBeDefined()
    expect(buttons.find(button => button.text() === '重新生成')!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('cancels pending requests when the associated user loses access', async () => {
    api.getDesktopMemberReports.mockReturnValue(new Promise(() => {}))
    api.getDesktopSummaryReport.mockReturnValue(new Promise(() => {}))
    const wrapper = render()
    const signal = api.getDesktopMemberReports.mock.calls[0][3] as AbortSignal
    expect(signal.aborted).toBe(false)
    await wrapper.setProps({ enabled: false }); expect(signal.aborted).toBe(true)
    expect(wrapper.find('input[type="date"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(30000)
    expect(api.getDesktopMemberReports).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('sanitizes AI Markdown and supports keyboard navigation', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, content: '<script>alert(1)</script><img src=x onerror="alert(2)">安全正文' } })
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('article script').exists()).toBe(false)
    expect(wrapper.find('article img').attributes('onerror')).toBeUndefined()
    await wrapper.get('#daily-report-tab-summary').trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.get('#daily-report-tab-members').attributes('aria-selected')).toBe('true')
    wrapper.unmount()
  })
  it('shows loading without reporting that a report was not generated', async () => {
    api.getDesktopMemberReports.mockReturnValue(new Promise(() => {}))
    api.getDesktopSummaryReport.mockReturnValue(new Promise(() => {}))
    const wrapper = render({ selfManaged: false })
    expect(wrapper.text()).toContain('正在加载报表')
    expect(wrapper.text()).not.toContain('尚未生成')
    expect(wrapper.get('[role="tabpanel"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.findAll('button').find(button => button.text() === '按日期补跑')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('distinguishes an unavailable report from a failed query and offers refresh recovery', async () => {
    api.getDesktopSummaryReport.mockRejectedValueOnce(new Error('Network error'))
    const wrapper = render({ selfManaged: false }); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('刷新重试')
    expect(wrapper.text()).not.toContain('尚未生成')
    expect(wrapper.findAll('button').find(button => button.text() === '按日期补跑')!.attributes('disabled')).toBeDefined()
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('企业整体进展')
    wrapper.unmount()
  })
  it('shows pending generation separately from the previous available result and can pause polling', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'running', task: { id: 1, status: 'running' }, summary: report })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('所选日期的任务尚未完成')
    expect(wrapper.text()).toContain('个人工作产出')
    await wrapper.findAll('button').find(button => button.text() === '暂停自动刷新')!.trigger('click')
    await vi.advanceTimersByTimeAsync(30000)
    expect(api.getDesktopSummaryReport).toHaveBeenCalledTimes(1)
    await wrapper.findAll('button').find(button => button.text() === '恢复自动刷新')!.trigger('click'); await flushPromises()
    expect(api.getDesktopSummaryReport).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('shows an active task without a false not-generated empty state', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'pending', task: { id: 1, status: 'pending' }, summary: { ...report, content: '' } })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('待生成')
    expect(wrapper.text()).not.toContain('尚未生成')
    wrapper.unmount()
  })
  it('clears a member filter and keeps no-conversation members visible', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('input[type="search"]').setValue('不存在的成员')
    expect(wrapper.text()).toContain('清除筛选以查看全部成员')
    await wrapper.findAll('button').find(button => button.text() === '清除筛选')!.trigger('click')
    expect(wrapper.text()).toContain('成员二')
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    wrapper.unmount()
  })
  it('jumps from a missing member to report details with a recovery instruction', async () => {
    api.getDesktopMemberReport.mockResolvedValue({ ...member, status: 'failed', content: '', error: 'upstream_failed' })
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'partial', missing_members: ['mem_one'], summary: report })
    api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [{ ...member, status: 'failed', content: '', error: 'upstream_failed' }, empty] })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '成员一')!.trigger('click')
    expect(wrapper.get('#daily-report-tab-members').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('#daily-report-member-heading').text()).toContain('成员一')
    expect(wrapper.text()).toContain('AI 调用失败')
    expect(wrapper.text()).toContain('联系平台管理员')
    wrapper.unmount()
  })
  it('rejects an empty date without leaving loading active or submitting a task', async () => {
    const wrapper = render({ selfManaged: false }); await flushPromises()
    await wrapper.get('input[type="date"]').setValue(''); await flushPromises()
    expect(wrapper.get('input[type="date"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.text()).toContain('选择前日或更早')
    expect(wrapper.get('[role="tabpanel"]').attributes('aria-busy')).toBe('false')
    expect(api.getDesktopSummaryReport).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('button').find(button => button.text() === '按日期补跑')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

})
