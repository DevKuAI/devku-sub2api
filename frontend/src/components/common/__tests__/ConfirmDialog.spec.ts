import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfirmDialog from '../ConfirmDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('ConfirmDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('uses the active locale for the close button even without a content class', async () => {
    const wrapper = mount(ConfirmDialog, {
      props: { show: true, title: 'Delete attachment', message: 'The attachment will no longer be shown.' },
      global: { stubs: { Icon: true, teleport: true } },
    })
    await wrapper.get('button[aria-label="common.close"]').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    wrapper.unmount()
  })

  it('disables cancellation and duplicate confirmation while loading', async () => {
    const wrapper = mount(ConfirmDialog, {
      attachTo: document.body,
      props: {
        show: true,
        title: 'Rotate token',
        message: 'Existing authorization will stop working.',
        confirmText: 'Rotate',
        loading: true
      },
      global: { stubs: { Icon: true } }
    })

    const buttons = document.body.querySelectorAll<HTMLButtonElement>('button')
    expect(buttons).toHaveLength(2)
    expect(Array.from(buttons).every((button) => button.disabled)).toBe(true)

    await buttons[1].click()
    await buttons[0].click()
    expect(wrapper.emitted('confirm')).toBeUndefined()
    expect(wrapper.emitted('cancel')).toBeUndefined()
    wrapper.unmount()
  })
})
