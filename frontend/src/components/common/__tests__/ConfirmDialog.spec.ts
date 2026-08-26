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
