import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { DesktopConversation, DesktopConversationDetail } from '@/api/desktopConversations'

const api = vi.hoisted(() => ({ listDesktopConversations: vi.fn(), getDesktopConversation: vi.fn(), getDesktopConversationStatistics: vi.fn() }))
vi.mock('@/api/desktopConversations', () => api)
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopConversationRecords from '../DesktopConversationRecords.vue'
import DesktopConversationText from '../DesktopConversationText.vue'

const record: DesktopConversation = {
  record_id: 'record_one', organization_id: 'org_one', member_id: 'mem_one', member_name: 'Member', member_deleted: false,
  client: 'workbuddy', installation_id: 'device_one', source_session_id: 'source_one', source_turn_id: null,
  started_at: '2026-09-18T00:00:00Z', stopped_at: '2026-09-18T00:01:00Z', received_at: '2026-09-18T00:01:01Z',
  duration_ms: 60_000,
  cwd: '/workspace', capture_status: 'captured', prompt_count: 1,
}
const detail: DesktopConversationDetail = { ...record, schema_version: 2, prompts: [{ text: '**提问**\n\n<script>private</script>', truncated: true }], response: { text: '## answer\n\n- `origin/main`', truncated: false } }
const DataTableStub = defineComponent({
  props: ['data', 'columns', 'loading'],
  template: '<div><slot v-if="!loading && !data.length" name="empty" /><div v-for="row in data" :key="row.record_id"><slot name="cell-member_name" :row="row" /><slot name="cell-duration_ms" :value="row.duration_ms" /><slot name="cell-actions" :row="row" /></div></div>',
})
const counts = (recordCount: number, promptCount: number) => ({ record_count: recordCount, prompt_count: promptCount, duration_record_count: recordCount, total_duration_ms: recordCount * 60_000, average_duration_ms: recordCount ? 60_000 : null })
function view() {
  return mount(DesktopConversationRecords, { props: { organizationId: 'org_one', selfManaged: false }, global: { stubs: { DataTable: DataTableStub, Pagination: true, teleport: true } } })
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

describe('Desktop conversation records', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listDesktopConversations.mockResolvedValue({ items: [record], total: 1, page: 1, page_size: 20, pages: 1 })
    api.getDesktopConversation.mockResolvedValue(detail)
    api.getDesktopConversationStatistics.mockResolvedValue({ timezone: 'Asia/Shanghai', as_of: '2026-09-22T04:00:00Z', today: counts(3, 5), week: counts(12, 20), month: counts(30, 45), total: counts(80, 100), last_30_days: counts(30, 45), captured_last_30_days: 25, response_missing_last_30_days: 5, workbuddy_last_30_days: 10, chatgpt_codex_last_30_days: 20 })
  })

  it('loads only metadata until selected, renders Markdown safely, and preserves missing responses', async () => {
    const wrapper = view()
    await flushPromises()
    expect(api.getDesktopConversation).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="conversation-duration"]').text()).toBe('1m 0s')
    await wrapper.findAll('button').find((button) => button.text().includes('viewDetail'))!.trigger('click')
    await flushPromises()
    expect(wrapper.get('.conversation-markdown strong').text()).toBe('提问')
    expect(wrapper.get('.conversation-markdown').text()).toContain('<script>private</script>')
    expect(wrapper.get('.conversation-markdown h2').text()).toBe('answer')
    expect(wrapper.get('.conversation-markdown li code').text()).toBe('origin/main')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('conversations.truncated')
    api.getDesktopConversation.mockResolvedValue({ ...detail, response: null, capture_status: 'response_missing' })
    await wrapper.findAll('button').find((button) => button.text().includes('viewDetail'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('conversations.responseMissing')
    wrapper.unmount()
  })

  it('shows an unknown duration for historical records', async () => {
    api.listDesktopConversations.mockResolvedValue({ items: [{ ...record, duration_ms: null }], total: 1, page: 1, page_size: 20, pages: 1 })
    api.getDesktopConversation.mockResolvedValue({ ...detail, duration_ms: null })
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[data-testid="conversation-statistics"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="conversation-duration"]').text()).toBe('—')
    await wrapper.findAll('button').find(button => button.text().includes('viewDetail'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').text()).toContain('—')
    wrapper.unmount()
  })

  it('anchors thread queries to member, client, installation and source ID', async () => {
    const wrapper = view()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('viewThread'))!.trigger('click')
    await flushPromises()
    expect(api.listDesktopConversations).toHaveBeenLastCalledWith('org_one', false, {
      page: 1, page_size: 20, sort_order: 'asc', member_id: 'mem_one', client: 'workbuddy', installation_id: 'device_one', source_session_id: 'source_one',
    }, expect.any(AbortSignal))
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('conversations.threadHint')
    expect(api.getDesktopConversation).toHaveBeenCalledWith('org_one', false, record.record_id, expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('cancels and discards a detail response when switching organization', async () => {
    const pending = deferred<DesktopConversationDetail>()
    api.getDesktopConversation.mockReturnValue(pending.promise)
    const wrapper = view()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('viewDetail'))!.trigger('click')
    const signal = api.getDesktopConversation.mock.calls[0][3] as AbortSignal
    api.listDesktopConversations.mockResolvedValue({ items: [], total: 0 })
    await wrapper.setProps({ organizationId: 'org_two' })
    expect(signal.aborted).toBe(true)
    pending.resolve(detail)
    await flushPromises()
    expect(wrapper.find('[data-testid="conversation-detail"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('private')
    wrapper.unmount()
  })

  it('discards stale list responses and keeps an empty new organization empty', async () => {
    const pending = deferred<{ items: DesktopConversation[]; total: number }>()
    api.listDesktopConversations.mockReturnValueOnce(pending.promise)
    const wrapper = view()
    const signal = api.listDesktopConversations.mock.calls[0][3] as AbortSignal
    api.listDesktopConversations.mockResolvedValue({ items: [], total: 0 })
    await wrapper.setProps({ organizationId: 'org_two' })
    pending.resolve({ items: [record], total: 1 })
    await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(wrapper.text()).toContain('conversations.empty')
    expect(wrapper.text()).not.toContain('Member')
    wrapper.unmount()
  })

  it('shows errors and preserves deleted member history', async () => {
    api.listDesktopConversations.mockRejectedValueOnce(new Error('offline'))
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('conversations.loadFailed')
    api.listDesktopConversations.mockResolvedValue({ items: [{ ...record, member_deleted: true }], total: 1 })
    await wrapper.find('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('conversations.deletedMember')
    wrapper.unmount()
  })

  it('keeps list filters and pagination when browsing and closing a conversation', async () => {
    const wrapper = view()
    await flushPromises()
    await wrapper.find('input[type="search"]').setValue('Member')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 3)
    await flushPromises()
    const callsBeforeOpen = api.listDesktopConversations.mock.calls.length
    await wrapper.findAll('button').find(button => button.text().includes('viewThread'))!.trigger('click')
    await flushPromises()
    await wrapper.find('[role="dialog"] button[aria-label="common.close"]').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect((wrapper.find('input[type="search"]').element as HTMLInputElement).value).toBe('Member')
    expect(wrapper.findComponent({ name: 'Pagination' }).props('page')).toBe(3)
    expect(api.listDesktopConversations).toHaveBeenCalledTimes(callsBeforeOpen + 1)
    wrapper.unmount()
  })

  it('navigates in the dialog and discards late detail responses', async () => {
    const second = { ...record, record_id: 'record_two' }
    api.listDesktopConversations.mockResolvedValue({ items: [record, second], total: 2 })
    const pending = deferred<DesktopConversationDetail>()
    api.getDesktopConversation.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ ...detail, ...second, prompts: [{ text: 'second question', truncated: false }], response: { text: 'second answer', truncated: false } })
    const wrapper = view()
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('viewDetail'))!.trigger('click')
    const signal = api.getDesktopConversation.mock.calls[0][3] as AbortSignal
    expect(wrapper.find('[role="dialog"]').text()).toContain('common.loading')
    await wrapper.findAll('[role="dialog"] button').find(button => button.text().includes('nextRecord'))!.trigger('click')
    await flushPromises()
    expect(signal.aborted).toBe(true)
    pending.resolve(detail)
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').text()).toContain('second answer')
    expect(wrapper.find('[role="dialog"]').text()).not.toContain('private')
    expect(wrapper.findAll('[role="dialog"] button').find(button => button.text().includes('nextRecord'))!.attributes('disabled')).toBeDefined()
    expect(wrapper.find('details').attributes('open')).toBeUndefined()
    wrapper.unmount()
  })

  it('cancels pending thread requests when the dialog closes', async () => {
    const wrapper = view()
    await flushPromises()
    const pending = deferred<{ items: DesktopConversation[]; total: number }>()
    api.listDesktopConversations.mockReturnValueOnce(pending.promise)
    await wrapper.findAll('button').find(button => button.text().includes('viewThread'))!.trigger('click')
    const signal = api.listDesktopConversations.mock.calls[1][3] as AbortSignal
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(signal.aborted).toBe(true)
    pending.resolve({ items: [record], total: 1 })
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.getDesktopConversation).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('loads the next conversation page while keeping the original scope for enterprise users', async () => {
    const wrapper = view()
    await wrapper.setProps({ selfManaged: true })
    await flushPromises()
    const firstPage = Array.from({ length: 20 }, (_, index) => ({ ...record, record_id: `thread_${index}` }))
    const last = { ...record, record_id: 'last_record' }
    api.listDesktopConversations.mockResolvedValueOnce({ items: firstPage, total: 21 }).mockResolvedValueOnce({ items: [last], total: 21 })
    await wrapper.findAll('button').find(button => button.text().includes('viewThread'))!.trigger('click')
    await flushPromises()
    const select = wrapper.find('[role="dialog"] select')
    await select.setValue('thread_19')
    await flushPromises()
    await wrapper.findAll('[role="dialog"] button').find(button => button.text().includes('nextRecord'))!.trigger('click')
    await flushPromises()
    expect(api.listDesktopConversations).toHaveBeenLastCalledWith('org_one', true, {
      page: 2, page_size: 20, sort_order: 'asc', member_id: 'mem_one', client: 'workbuddy', installation_id: 'device_one', source_session_id: 'source_one',
    }, expect.any(AbortSignal))
    expect(api.getDesktopConversation).toHaveBeenLastCalledWith('org_one', true, last.record_id, expect.any(AbortSignal))
    expect(wrapper.find('[role="dialog"]').text()).toContain('21 / 21')
    wrapper.unmount()
  })

  it('retries thread and detail failures and handles empty conversations', async () => {
    const wrapper = view()
    await flushPromises()
    api.listDesktopConversations.mockRejectedValueOnce(new Error('offline'))
    await wrapper.findAll('button').find(button => button.text().includes('viewThread'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"] [role="alert"]').text()).toContain('conversations.loadFailed')
    api.getDesktopConversation.mockRejectedValueOnce({ response: { status: 404 } })
    await wrapper.find('[role="dialog"] [role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="conversation-detail"] [role="alert"]').text()).toContain('conversations.notFound')
    await wrapper.find('[data-testid="conversation-detail"] [role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"] .conversation-markdown').exists()).toBe(true)
    await wrapper.find('[role="dialog"] button[aria-label="common.close"]').trigger('click')
    api.listDesktopConversations.mockResolvedValueOnce({ items: [], total: 0 })
    await wrapper.findAll('button').find(button => button.text().includes('viewThread'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').text()).toContain('conversations.empty')
    expect(wrapper.findAll('[role="dialog"] button').find(button => button.text().includes('nextRecord'))!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('loads statistics across all pages and applies only submitted filters to both requests', async () => {
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[data-testid="statistics-total"]').text()).toContain('80')
    expect(wrapper.find('[data-testid="conversation-reporting-breakdown"]').text()).toContain('25')
    expect(api.getDesktopConversationStatistics).toHaveBeenLastCalledWith('org_one', false, {}, expect.any(AbortSignal))
    const initialCalls = api.getDesktopConversationStatistics.mock.calls.length
    await wrapper.find('input[type="search"]').setValue('  Member  ')
    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 2)
    await flushPromises()
    expect(api.getDesktopConversationStatistics).toHaveBeenCalledTimes(initialCalls)
    expect(api.listDesktopConversations.mock.lastCall?.[2]).not.toHaveProperty('member_search')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.getDesktopConversationStatistics).toHaveBeenLastCalledWith('org_one', false, { member_search: 'Member' }, expect.any(AbortSignal))
    expect(api.listDesktopConversations.mock.lastCall?.[2]).toMatchObject({ member_search: 'Member', page: 1 })
    await wrapper.findAll('form button').find(button => button.text().includes('reset'))!.trigger('click')
    await flushPromises()
    expect(api.getDesktopConversationStatistics.mock.lastCall?.[2]).toEqual({})
    wrapper.unmount()
  })

  it('preserves record browsing when statistics fail and retries independently', async () => {
    api.getDesktopConversationStatistics.mockRejectedValueOnce(new Error('offline'))
    const wrapper = view()
    await flushPromises()
    const statistics = wrapper.find('[data-testid="conversation-statistics"]')
    expect(statistics.text()).toContain('statistics.loadFailed')
    expect(wrapper.findAll('button').some(button => button.text().includes('viewDetail'))).toBe(true)
    const listCalls = api.listDesktopConversations.mock.calls.length
    await statistics.find('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(statistics.find('[data-testid="statistics-today"]').text()).toContain('3')
    expect(api.listDesktopConversations).toHaveBeenCalledTimes(listCalls)
    wrapper.unmount()
  })

  it('discards stale statistics after changing organization or role and displays empty counts', async () => {
    const pending = deferred<unknown>()
    api.getDesktopConversationStatistics.mockReturnValueOnce(pending.promise)
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[data-testid="statistics-total"]').text()).toContain('—')
    const signal = api.getDesktopConversationStatistics.mock.calls[0][3] as AbortSignal
    const empty = { timezone: 'UTC', as_of: '2026-09-22T04:00:00Z', today: counts(0, 0), week: counts(0, 0), month: counts(0, 0), total: counts(0, 0), last_30_days: counts(0, 0), captured_last_30_days: 0, response_missing_last_30_days: 0, workbuddy_last_30_days: 0, chatgpt_codex_last_30_days: 0 }
    api.getDesktopConversationStatistics.mockResolvedValue(empty)
    await wrapper.setProps({ organizationId: 'org_two', selfManaged: true })
    await flushPromises()
    pending.resolve({ ...empty, total: { record_count: 999, prompt_count: 999 } })
    await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(api.getDesktopConversationStatistics).toHaveBeenLastCalledWith('org_two', true, {}, expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="statistics-total"]').text()).toContain('0')
    expect(wrapper.find('[data-testid="statistics-total"]').text()).not.toContain('999')
    wrapper.unmount()
    expect((api.getDesktopConversationStatistics.mock.lastCall?.[3] as AbortSignal).aborted).toBe(true)
  })

  it('renders Markdown blocks, inline formatting, line breaks and keyboard reachable tables', () => {
    const text = [
      '# 标题', '', '第一行', '第二行 **加粗** *强调* ~~删除~~ [文档](https://example.com/docs)', '',
      '- 项目 `origin/main`', '', '1. 步骤', '', '> 引用', '',
      '```html', '<div>code</div>', '```', '', '| 名称 | 状态 |', '| --- | --- |', '| Markdown | 支持 |',
    ].join('\n')
    const wrapper = mount(DesktopConversationText, { props: { segment: { text, truncated: false } } })
    expect(wrapper.get('h1').text()).toBe('标题')
    expect(wrapper.find('p br').exists()).toBe(true)
    expect(wrapper.get('strong').text()).toBe('加粗')
    expect(wrapper.get('em').text()).toBe('强调')
    expect(wrapper.get('del').text()).toBe('删除')
    expect(wrapper.get('a').attributes('href')).toBe('https://example.com/docs')
    expect(wrapper.get('ul li code').text()).toBe('origin/main')
    expect(wrapper.get('ol li').text()).toBe('步骤')
    expect(wrapper.get('blockquote').text()).toBe('引用')
    expect(wrapper.get('pre code.language-html').text()).toBe('<div>code</div>')
    expect(wrapper.get('.conversation-markdown-table').attributes()).toMatchObject({ tabindex: '0', role: 'region', 'aria-label': 'admin.desktop.detail.tableScrollHint' })
    expect(wrapper.findAll('tbody td').map(cell => cell.text())).toEqual(['Markdown', '支持'])
    wrapper.unmount()
  })

  it('keeps raw HTML as text and removes unsafe Markdown links', () => {
    const text = '<script>alert(1)</script>\n\n<img src="x" onerror="alert(2)">\n\n[危险链接](javascript:alert%283%29)\n\n`<iframe src="x">`'
    const wrapper = mount(DesktopConversationText, { props: { segment: { text, truncated: false } } })
    expect(wrapper.find('script, img, iframe').exists()).toBe(false)
    expect(wrapper.text()).toContain('<script>alert(1)</script>')
    expect(wrapper.text()).toContain('<img src="x" onerror="alert(2)">')
    expect(wrapper.get('a').attributes('href')).toBeUndefined()
    expect(wrapper.get('code').text()).toBe('<iframe src="x">')
    wrapper.unmount()
  })

  it('preserves the empty text placeholder', () => {
    const wrapper = mount(DesktopConversationText, { props: { segment: { text: '', truncated: false } } })
    expect(wrapper.get('.conversation-markdown').text()).toBe('admin.desktop.conversations.emptyText')
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })

  it('expands and collapses long Markdown without modifying its content', async () => {
    const text = `${'文'.repeat(3000)}end`
    const wrapper = mount(DesktopConversationText, { props: { segment: { text, truncated: false } } })
    expect(wrapper.get('.conversation-markdown').text().length).toBe(2400)
    await wrapper.find('button').trigger('click')
    expect(wrapper.get('.conversation-markdown').text()).toBe(text)
    expect(wrapper.find('button').attributes('aria-expanded')).toBe('true')
    await wrapper.find('button').trigger('click')
    expect(wrapper.get('.conversation-markdown').text().length).toBe(2400)
    expect(wrapper.find('button').attributes('aria-expanded')).toBe('false')
    wrapper.unmount()
  })
  it('explains an inverted received-time range before making a request', async () => {
    const wrapper = view(); await flushPromises()
    const count = api.listDesktopConversations.mock.calls.length
    const dates = wrapper.findAll('input[type="datetime-local"]')
    await dates[0].setValue('2026-10-10T10:00')
    await dates[1].setValue('2026-10-09T10:00')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('#conversation-filter-error').text()).toBe('admin.desktop.detail.invalidDateRange')
    expect(dates[0].attributes('aria-invalid')).toBe('true')
    expect(api.listDesktopConversations).toHaveBeenCalledTimes(count)
    await dates[0].setValue('2026-10-08T10:00')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('#conversation-filter-error').exists()).toBe(false)
    expect(api.listDesktopConversations).toHaveBeenCalledTimes(count + 1)
    wrapper.unmount()
  })

})
