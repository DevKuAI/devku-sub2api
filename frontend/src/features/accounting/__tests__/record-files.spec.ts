import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, record } from './testing'

const api = vi.hoisted(() => ({
  attachments: vi.fn(), upload: vi.fn(), download: vi.fn(), mutate: vi.fn(),
  stableRequest: () => Object.assign(() => 'key', { reset: vi.fn() }),
}))
vi.mock('../api', () => api)
import RecordFiles from '../components/RecordFiles.vue'

enableAutoUnmount(afterEach)
const proof = new File(['proof'], '付款凭证.pdf', { type: 'application/pdf' })
async function render(props = {}) {
  const wrapper = mount(RecordFiles, { props: { kind: 'transfers', record: record(), ...props }, global: { plugins: [i18n()], stubs: { ConfirmDialog: true } } })
  await flushPromises()
  return wrapper
}
async function select(wrapper: Awaited<ReturnType<typeof render>>) {
  const input = wrapper.get('input[type="file"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: [proof] })
  await input.trigger('change'); await flushPromises()
}

describe('accounting attachment uploads', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.attachments.mockResolvedValue([])
    api.upload.mockResolvedValue({ id: 'file-one' })
  })

  it('uploads immediately, refreshes saved attachments and clears the pending filename', async () => {
    const wrapper = await render()
    api.attachments.mockResolvedValueOnce([{ id: 'file-one', filename: proof.name, size_bytes: proof.size, status: 'COMPLETE', customer_visible: false }])
    await select(wrapper)
    expect(api.upload).toHaveBeenCalledWith(proof, 'transfers', expect.objectContaining({ id: 'record-one' }))
    expect(wrapper.get('li').text()).toContain(proof.name)
    expect(wrapper.text()).toContain('未选择文件')
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('keeps failed uploads selected and allows retrying the same file', async () => {
    api.upload.mockRejectedValueOnce({ message: '上传失败' })
    const wrapper = await render()
    await select(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toBe('上传失败')
    expect(wrapper.text()).toContain(proof.name)
    expect(wrapper.emitted('changed')).toBeUndefined()
    await select(wrapper)
    expect(api.upload).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('未选择文件')
  })

  it.each([{ customer: true }, { record: record({ status: 'VOID' }) }])('keeps read-only records free of upload controls: %o', async props => {
    const wrapper = await render(props)
    expect(wrapper.find('input[type="file"]').exists()).toBe(false)
    expect(api.upload).not.toHaveBeenCalled()
  })
})
