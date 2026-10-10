import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import BaseDialog from '../BaseDialog.vue'

enableAutoUnmount(afterEach)
const pressEscape = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

describe('stacked dialog Escape handling', () => {
  it('closes only the most recently opened dialog, then allows the parent to close', async () => {
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent' } })
    const child = mount(BaseDialog, { props: { show: true, title: 'Child' } })
    pressEscape()
    expect(child.emitted('close')).toHaveLength(1)
    expect(parent.emitted('close')).toBeUndefined()
    await child.setProps({ show: false })
    pressEscape()
    expect(parent.emitted('close')).toHaveLength(1)
  })

  it('does not dismiss a parent behind a child that disallows Escape', () => {
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent' } })
    const child = mount(BaseDialog, { props: { show: true, title: 'Child', closeOnEscape: false } })
    pressEscape()
    expect(child.emitted('close')).toBeUndefined()
    expect(parent.emitted('close')).toBeUndefined()
    child.unmount()
    pressEscape()
    expect(parent.emitted('close')).toHaveLength(1)
  })
  it('keeps the dialog open when a nested control has consumed Escape', () => {
    const wrapper = mount(BaseDialog, { props: { show: true, title: 'Editor' } })
    const event = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true })
    event.preventDefault()
    document.dispatchEvent(event)
    expect(wrapper.emitted('close')).toBeUndefined()
  })
  it('makes the page and underlying dialog inert and restores them as dialogs close', async () => {
    const app = document.createElement('main'); app.id = 'app'; document.body.append(app)
    const parent = mount(BaseDialog, { props: { show: true, title: 'Parent' } })
    await parent.vm.$nextTick()
    const parentOverlay = [...document.querySelectorAll<HTMLElement>('[role="dialog"]')].find(el => el.textContent?.includes('Parent'))!
    const child = mount(BaseDialog, { props: { show: true, title: 'Child' } })
    await child.vm.$nextTick()
    expect(app.hasAttribute('inert')).toBe(true)
    expect(parentOverlay.hasAttribute('inert')).toBe(true)
    await child.setProps({ show: false })
    expect(app.hasAttribute('inert')).toBe(true)
    expect(parentOverlay.hasAttribute('inert')).toBe(false)
    await parent.setProps({ show: false })
    expect(app.hasAttribute('inert')).toBe(false)
    app.remove()
  })

})
