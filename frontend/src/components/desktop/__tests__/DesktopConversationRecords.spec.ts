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
  cwd: '/workspace', capture_status: 'captured', prompt_count: 1,
}
const detail: DesktopConversationDetail = { ...record, schema_version: 2, prompts: [{ text: '<script>private</script>', truncated: true }], response: { text: 'answer', truncated: false } }
const DataTableStub = defineComponent({
  props: ['data', 'columns', 'loading'],
  template: '<div><slot v-if="!loading && !data.length" name="empty" /><div v-for="row in data" :key="row.record_id"><slot name="cell-member_name" :row="row" /><slot name="cell-actions" :row="row" /></div></div>',
})
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
    api.getDesktopConversationStatistics.mockResolvedValue({ timezone: 'Asia/Shanghai', as_of: '2026-09-22T04:00:00Z', today: { record_count: 3, prompt_count: 5 }, week: { record_count: 12, prompt_count: 20 }, month: { record_count: 30, prompt_count: 45 }, total: { record_count: 80, prompt_count: 100 } })
  })

  it('loads only metadata until selected, renders plain text, and preserves missing responses', async () => {
    const wrapper = view()
    await flushPromises()
    expect(api.getDesktopConversation).not.toHaveBeenCalled()
    await wrapper.findAll('button').find((button) => button.text().includes('viewDetail'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('pre').text()).toBe('<script>private</script>')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('conversations.truncated')
    api.getDesktopConversation.mockResolvedValue({ ...detail, response: null, capture_status: 'response_missing' })
    await wrapper.findAll('button').find((button) => button.text().includes('viewDetail'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('conversations.responseMissing')
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
    await wrapper.find('[role="dialog"] button[aria-label="Close modal"]').trigger('click')
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
    expect(wrapper.find('[role="dialog"] pre').exists()).toBe(true)
    await wrapper.find('[role="dialog"] button[aria-label="Close modal"]').trigger('click')
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
    const empty = { timezone: 'UTC', as_of: '2026-09-22T04:00:00Z', today: { record_count: 0, prompt_count: 0 }, week: { record_count: 0, prompt_count: 0 }, month: { record_count: 0, prompt_count: 0 }, total: { record_count: 0, prompt_count: 0 } }
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

  it('expands long text without modifying its content', async () => {
    const text = `${'文'.repeat(3000)}end`
    const wrapper = mount(DesktopConversationText, { props: { segment: { text, truncated: false } } })
    expect(wrapper.find('pre').text().length).toBe(2400)
    await wrapper.find('button').trigger('click')
    expect(wrapper.find('pre').text()).toBe(text)
    expect(wrapper.find('button').attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })
})
