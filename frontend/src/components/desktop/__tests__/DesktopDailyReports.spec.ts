import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import zh from '@/i18n/locales/zh'
import DesktopDailyReports from '../DesktopDailyReports.vue'

const api = vi.hoisted(() => ({ getDesktopMemberReports: vi.fn(), getDesktopMemberReport: vi.fn(), getDesktopSummaryReport: vi.fn(), runDesktopReports: vi.fn(), getDesktopConversation: vi.fn() }))
vi.mock('@/api/desktopReports', () => api)
vi.mock('@/api/desktopConversations', () => ({ getDesktopConversation: api.getDesktopConversation }))
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
function render(overrides = {}, viewerStub = true) {
  return mount(DesktopDailyReports, { attachTo: document.body, props: { ...props, ...overrides }, global: { stubs: { DesktopConversationViewer: viewerStub, teleport: true } } })
}
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-10-10T04:00:00Z')); vi.clearAllMocks()
  api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [member, empty] })
  api.getDesktopMemberReport.mockResolvedValue(member)
  api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, content: '企业整体进展' } })
  api.runDesktopReports.mockResolvedValue({ status: 'pending' })
})
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

describe('DesktopDailyReports', () => {
  it('renders generated Markdown labels in the enterprise daily summary and copies the original text', async () => {
    const content = '# 企业管理日报总结\n\n**范围： **1 位成员。\n\n- **重点成果： **完成验证。'
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, content } })
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('article h1').text()).toBe('企业管理日报总结')
    expect(wrapper.findAll('article strong').map(node => node.text())).toEqual(['范围：', '重点成果：'])
    expect(wrapper.get('article').text()).not.toContain('**')
    await wrapper.findAll('button').find(button => button.text() === '复制')!.trigger('click'); await flushPromises()
    expect(writeText).toHaveBeenCalledWith(content)
    wrapper.unmount()
  })

  it('places summary text before model metadata and report coverage', async () => {
    const wrapper = render(); await flushPromises()
    const body = wrapper.get('article')
    for (const selector of ['.report-metadata', '.report-metrics']) {
      expect(body.element.compareDocumentPosition(wrapper.get(selector).element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    }
    wrapper.unmount()
  })

  it('copies the displayed summary only after rechecking current readable text', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '复制')!.trigger('click'); await flushPromises()
    expect(writeText).toHaveBeenCalledWith('企业整体进展')
    wrapper.unmount(); vi.unstubAllGlobals()
  })

  it('asks the reader to review new text instead of silently copying a changed summary', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
    const wrapper = render(); await flushPromises()
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, summary: { ...report, content: '新的总结正文' } })
    await wrapper.findAll('button').find(button => button.text() === '复制')!.trigger('click'); await flushPromises()
    expect(writeText).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('正文已更新')
    expect(wrapper.get('article').text()).toContain('新的总结正文')
    wrapper.unmount(); vi.unstubAllGlobals()
  })

  it('loads member text only after selecting a report', async () => {
    api.getDesktopMemberReports.mockResolvedValue({ ...base, total_members: 21, members: [{ ...member, content: '', source_ids: [], executions: [] }, empty] })
    const wrapper = render(); await flushPromises()
    expect(api.getDesktopMemberReport).not.toHaveBeenCalled()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    expect(wrapper.text()).not.toContain('个人工作产出')
    await wrapper.find('tbody button').trigger('click'); await flushPromises()
    expect(api.getDesktopMemberReport).toHaveBeenCalledWith('org_one', true, '2026-10-09', 'mem_one', expect.any(AbortSignal), 1)
    expect(wrapper.get('[role="dialog"]').text()).toContain('个人工作产出')
    expect(wrapper.get('#daily-report-panel-members').text()).not.toContain('个人工作产出')
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
    const historicalMember = { ...member, executions: [{ ...execution, id: 2, kind: 'member', member_id: 'mem_one', model: 'member-old-model' }] }
    api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [historicalMember, empty] })
    api.getDesktopMemberReport.mockResolvedValue(historicalMember)
    const wrapper = render({ analysisModel: 'new-configuration-model' }); await flushPromises()
    expect(wrapper.get('[data-testid="report-execution-history"]').text()).toContain('summary-old-model')
    expect(wrapper.text()).not.toContain('member-old-model')
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.find('tbody button').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-testid="report-execution-history"]').text()).toContain('member-old-model')
    expect(wrapper.text()).not.toContain('summary-old-model')
    wrapper.unmount()
  })
  it('shows no-record day without allowing generation', async () => {
    api.getDesktopMemberReports.mockResolvedValue({ ...base, status: 'no_records', reason: 'no_records', expected_members: 0, completed_members: 0, members: [empty] })
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'no_records', reason: 'no_records', expected_members: 0, completed_members: 0, summary: { ...report, status: 'no_records', content: '' } })
    const wrapper = render({ selfManaged: false }); await flushPromises()
    expect(wrapper.text()).toContain('当天没有已上报对话，未生成日总结')
    const generate = wrapper.findAll('button').find(button => button.text() === '补生成日报')!
    expect(generate.attributes('disabled')).toBeDefined(); expect(api.runDesktopReports).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows model unavailability while keeping historical content', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, reason: 'model_unavailable', summary: { ...report, content: '历史总结' } })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('分析模型暂不可用'); expect(wrapper.text()).toContain('历史总结')
    expect(wrapper.text()).not.toContain('补生成日报')
    wrapper.unmount()
  })
  it('requires regeneration after analysis configuration changes', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, reason: 'configuration_changed', summary: report })
    const wrapper = render({ selfManaged: false }); await flushPromises()
    const buttons = wrapper.findAll('button')
    expect(buttons.find(button => button.text() === '补生成日报')!.attributes('disabled')).toBeDefined()
    expect(buttons.find(button => button.text() === '重试失败任务')!.attributes('disabled')).toBeDefined()
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
    expect(wrapper.text()).toContain('正在加载日报与总结')
    expect(wrapper.text()).not.toContain('尚未生成')
    expect(wrapper.get('[role="tabpanel"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.findAll('button').find(button => button.text() === '补生成日报')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('distinguishes an unavailable report from a failed query and offers refresh recovery', async () => {
    api.getDesktopSummaryReport.mockRejectedValueOnce(new Error('Network error'))
    const wrapper = render({ selfManaged: false }); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('刷新重试')
    expect(wrapper.text()).not.toContain('尚未生成')
    expect(wrapper.findAll('button').find(button => button.text() === '补生成日报')!.attributes('disabled')).toBeDefined()
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
  it('opens a missing member report in a dialog while retaining the summary tab', async () => {
    api.getDesktopMemberReport.mockResolvedValue({ ...member, status: 'failed', content: '', error: 'upstream_failed' })
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, status: 'partial', missing_members: ['mem_one'], summary: report })
    api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [{ ...member, status: 'failed', content: '', error: 'upstream_failed' }, empty] })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '成员一')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('#daily-report-tab-summary').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[role="dialog"] .modal-title').text()).toContain('成员一')
    expect(wrapper.text()).toContain('AI 调用失败')
    expect(wrapper.text()).toContain('联系平台管理员')
    wrapper.unmount()
  })
  it('rejects an empty date without leaving loading active or submitting a task', async () => {
    const wrapper = render({ selfManaged: false }); await flushPromises()
    await wrapper.get('input[type="date"]').setValue(''); await flushPromises()
    expect(wrapper.get('input[type="date"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.text()).toContain('请选择昨天或更早的日期')
    expect(wrapper.get('[role="tabpanel"]').attributes('aria-busy')).toBe('false')
    expect(api.getDesktopSummaryReport).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('button').find(button => button.text() === '补生成日报')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('closes with Escape, restores focus and retains the member filter', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('input[type="search"]').setValue('成员一'); await flushPromises()
    const trigger = wrapper.get('tbody button')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"]').element.contains(document.activeElement)).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
    expect((wrapper.get('input[type="search"]').element as HTMLInputElement).value).toBe('成员一')
    expect(wrapper.get('#daily-report-tab-members').attributes('aria-selected')).toBe('true')
    wrapper.unmount()
  })

  it('immediately opens a loading dialog for a member outside the current list and ignores a late response after closing', async () => {
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, missing_members: ['mem_other'], summary: report })
    let resolveDetail!: (value: typeof member) => void
    api.getDesktopMemberReport.mockReturnValue(new Promise(resolve => { resolveDetail = resolve }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'mem_other')!.trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('正在加载日报与总结')
    expect(wrapper.get('[role="dialog"] [aria-busy]').attributes('aria-busy')).toBe('true')
    const signal = api.getDesktopMemberReport.mock.calls[0][4] as AbortSignal
    await wrapper.get('[role="dialog"] button[aria-label="关闭"]').trigger('click')
    expect(signal.aborted).toBe(true)
    resolveDetail({ ...member, member_id: 'mem_other' }); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.get('#daily-report-tab-summary').attributes('aria-selected')).toBe('true')
    wrapper.unmount()
  })

  it('shows a detail request failure and retry inside the dialog', async () => {
    api.getDesktopMemberReports.mockResolvedValue({ ...base, members: [{ ...member, content: '' }] })
    api.getDesktopMemberReport.mockRejectedValueOnce(new Error('Network error'))
    const wrapper = render(); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('tbody button').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toContain('刷新重试')
    expect(wrapper.get('#daily-report-panel-members').find('[role="alert"]').exists()).toBe(false)
    await wrapper.get('[role="dialog"] [role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('个人工作产出')
    expect(wrapper.find('[role="dialog"] [role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each(['date', 'organization', 'access'])('closes the member dialog and aborts its request when %s changes', async (change) => {
    api.getDesktopMemberReport.mockReturnValue(new Promise(() => {}))
    const wrapper = render(); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('tbody button').trigger('click')
    const signal = api.getDesktopMemberReport.mock.calls[0][4] as AbortSignal
    if (change === 'date') await wrapper.get('input[type="date"]').setValue('2026-10-08')
    else await wrapper.setProps(change === 'organization' ? { organizationId: 'org_two' } : { enabled: false })
    await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps member execution pagination independent of the summary beneath the dialog', async () => {
    const executions = Array.from({ length: 50 }, (_, id) => ({ id, revision: 1, kind: 'summary', model: 'test-model', round: 0, chunk: 1, attempt: 1, request_id: `request-${id}`, status: 'completed', error: '', started_at: null, finished_at: null }))
    api.getDesktopSummaryReport.mockResolvedValue({ ...base, missing_members: ['mem_one'], summary: { ...report, executions } })
    api.getDesktopMemberReport.mockResolvedValue({ ...member, executions })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('#daily-report-panel-summary button').find(button => button.text() === '下一页')!.trigger('click'); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '成员一')!.trigger('click'); await flushPromises()
    expect(api.getDesktopMemberReport.mock.calls[0][5]).toBe(1)
    await wrapper.findAll('[role="dialog"] button').find(button => button.text() === '下一页')!.trigger('click'); await flushPromises()
    expect(api.getDesktopMemberReport.mock.lastCall?.[5]).toBe(2)
    expect(api.getDesktopSummaryReport).toHaveBeenCalledTimes(2)
    await wrapper.get('[role="dialog"] button[aria-label="关闭"]').trigger('click')
    expect(wrapper.findAll('#daily-report-panel-summary button').some(button => button.text() === '上一页')).toBe(true)
    wrapper.unmount()
  })

  it('keeps source failures in the dialog and cancels a pending source request on close', async () => {
    api.getDesktopConversation.mockRejectedValueOnce(new Error('Network error'))
    const wrapper = render(); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('tbody button').trigger('click'); await flushPromises()
    const sourceButton = wrapper.get('[role="dialog"] section button')
    await sourceButton.trigger('click'); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toContain(zh.admin.desktop.reports.sourceFailed)
    let resolveSource!: (value: object) => void
    api.getDesktopConversation.mockReturnValueOnce(new Promise(resolve => { resolveSource = resolve }))
    await sourceButton.trigger('click')
    const signal = api.getDesktopConversation.mock.lastCall?.[3] as AbortSignal
    await wrapper.get('[role="dialog"] button[aria-label="关闭"]').trigger('click')
    expect(signal.aborted).toBe(true)
    resolveSource({ public_id: 'record-one' }); await flushPromises()
    expect(wrapper.getComponent({ name: 'DesktopConversationViewer' }).props('record')).toBe(null)
    wrapper.unmount()
  })

  it('returns focus to the source button after closing the nested conversation dialog', async () => {
    const record = { record_id: 'record-one', organization_id: 'org_one', member_id: 'mem_one', member_name: '成员一', member_deleted: false, client: 'chatgpt_codex', installation_id: 'fixture', source_session_id: 'session-one', source_turn_id: null, started_at: '2026-10-09T02:00:00Z', stopped_at: '2026-10-09T02:00:01Z', duration_ms: 1000, received_at: '2026-10-09T02:00:01Z', cwd: null, capture_status: 'captured', prompt_count: 1, schema_version: 2, prompts: [{ text: '工作内容', truncated: false }], response: { text: '完成结果', truncated: false } }
    let resolveSource!: (value: typeof record) => void
    api.getDesktopConversation.mockReturnValueOnce(new Promise(resolve => { resolveSource = resolve })).mockResolvedValue(record)
    const wrapper = render({}, false); await flushPromises()
    await wrapper.get('#daily-report-tab-members').trigger('click')
    await wrapper.get('tbody button').trigger('click'); await flushPromises()
    const sourceButton = wrapper.get('[role="dialog"] section button')
    await sourceButton.trigger('click')
    ;(sourceButton.element as HTMLButtonElement).blur()
    resolveSource(record); await flushPromises()
    expect(wrapper.findAll('[role="dialog"]')).toHaveLength(2)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await flushPromises()
    expect(wrapper.findAll('[role="dialog"]')).toHaveLength(1)
    expect(document.activeElement).toBe(sourceButton.element)
    wrapper.unmount()
  })

})
