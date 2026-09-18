import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { DesktopConversation, DesktopConversationDetail } from '@/api/desktopConversations'

const api = vi.hoisted(() => ({ listDesktopConversations: vi.fn(), getDesktopConversation: vi.fn() }))
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
  return mount(DesktopConversationRecords, { props: { organizationId: 'org_one', selfManaged: false }, global: { stubs: { DataTable: DataTableStub, Pagination: true } } })
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
    expect(wrapper.find('[data-testid="thread-scope"]').exists()).toBe(true)
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
