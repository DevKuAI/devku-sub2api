import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { h, nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

describe('HelpTooltip', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps a hover tooltip open while the pointer moves between the trigger and the tooltip', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'copyable details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave', { relatedTarget: tooltip })
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: trigger.element }))
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: null }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('escapes a clipped ancestor, stays in the viewport, and uses viewport coordinates when scrolled', async () => {
    vi.stubGlobal('innerWidth', 320)
    vi.stubGlobal('innerHeight', 640)
    vi.stubGlobal('scrollX', 400)
    vi.stubGlobal('scrollY', 600)
    const host = document.createElement('div')
    host.style.overflow = 'hidden'
    document.body.append(host)
    const wrapper = mount(HelpTooltip, { attachTo: host, props: { content: 'recovery date' } })
    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue(new DOMRect(300, 12, 20, 24))
    vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 224, 60))
    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.parentElement).toBe(document.body)
    expect(host.contains(tooltip)).toBe(false)
    expect(parseFloat(tooltip.style.left)).toBeGreaterThanOrEqual(8)
    expect(parseFloat(tooltip.style.left) + 224).toBeLessThanOrEqual(320)
    expect(parseFloat(tooltip.style.top)).toBeGreaterThanOrEqual(36)
    expect(parseFloat(tooltip.style.top) + 60).toBeLessThanOrEqual(640)
    wrapper.unmount()
  })

  it('shows the associated description on focus, retains it on pointer leave, and dismisses on Escape', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { content: 'recovery date' },
      slots: { trigger: ({ tooltipId }) => h('button', { type: 'button', 'aria-describedby': tooltipId }, '429') },
    })
    const button = wrapper.get('button')
    button.element.focus()
    await nextTick()
    const tooltip = getTooltipElement()
    expect(button.attributes('aria-describedby')).toBe(tooltip.id)
    expect(tooltip.style.display).not.toBe('none')
    await wrapper.get('.group').trigger('mouseleave')
    expect(tooltip.style.display).not.toBe('none')
    await button.trigger('keydown', { key: 'Escape' })
    expect(tooltip.style.display).toBe('none')
    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })
})
