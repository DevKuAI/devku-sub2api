import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountingFileUpload from '../components/AccountingFileUpload.vue'
import { i18n } from './testing'

enableAutoUnmount(afterEach)
const proof = new File(['proof'], '付款凭证.pdf', { type: 'application/pdf' })
function render(props = {}) {
  return mount(AccountingFileUpload, { props, global: { plugins: [i18n()] } })
}

describe('accounting file selection', () => {
  it('opens the native file chooser from a keyboard accessible button', async () => {
    const wrapper = render()
    const input = wrapper.get('input[type="file"]')
    const click = vi.spyOn(input.element as HTMLInputElement, 'click')
    const button = wrapper.get('button')
    expect(button.attributes('type')).toBe('button')
    expect(button.text()).toBe('选择文件')
    expect(wrapper.text()).toContain('未选择文件')
    expect(wrapper.text()).toContain('20 MiB')
    await button.trigger('click')
    expect(click).toHaveBeenCalledOnce()
  })

  it('emits files, allows reselection of the same file and clears the selection explicitly', async () => {
    const wrapper = render()
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [proof] })
    await input.trigger('change')
    await wrapper.setProps({ modelValue: proof })
    expect(wrapper.text()).toContain(proof.name)
    expect(wrapper.text()).toContain('5 Bytes')
    expect(wrapper.get('button').text()).toBe('更换文件')
    expect((input.element as HTMLInputElement).value).toBe('')
    await input.trigger('change')
    await wrapper.get(`button[aria-label="移除 ${proof.name}"]`).trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[proof], [proof], [null]])
  })

  it('keeps the selected file when the chooser is cancelled', async () => {
    const wrapper = render({ modelValue: proof })
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [] })
    await input.trigger('change')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).toContain(proof.name)
  })

  it.each([{ disabled: true }, { busy: true }])('locks file changes while unavailable: %o', async props => {
    const wrapper = render({ modelValue: proof, ...props })
    expect(wrapper.findAll('button').every(button => button.attributes('disabled') !== undefined)).toBe(true)
    const input = wrapper.get('input[type="file"]')
    expect(input.attributes('disabled')).toBeDefined()
    Object.defineProperty(input.element, 'files', { value: [proof] })
    await input.trigger('change')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    if ('busy' in props) expect(wrapper.text()).toContain('上传中')
  })
})
