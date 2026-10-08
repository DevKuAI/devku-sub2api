import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopAnalyticsRangePicker from '../DesktopAnalyticsRangePicker.vue'

describe('Desktop analytics range picker', () => {
  it('selects presets and applies a valid custom range', async () => {
    const wrapper = mount(DesktopAnalyticsRangePicker, { props: { modelValue: { days: 30 } } })
    await wrapper.findAll('[role="group"] button')[0].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ days: 7 }])
    await wrapper.findAll('[role="group"] button')[3].trigger('click')
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2026-09-01')
    await dates[1].setValue('2026-09-07')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([{ from: '2026-09-01', to: '2026-09-07' }])
  })

  it('keeps the applied range when custom dates are reversed or exceed 90 days', async () => {
    const wrapper = mount(DesktopAnalyticsRangePicker, { props: { modelValue: { days: 30 } } })
    await wrapper.findAll('[role="group"] button')[3].trigger('click')
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2026-09-07')
    await dates[1].setValue('2026-09-01')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    await dates[0].setValue('2026-01-01')
    await dates[1].setValue('2026-09-01')
    await wrapper.find('form').trigger('submit')
    await dates[0].setValue('9999-01-01')
    await dates[1].setValue('9999-01-02')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
  it('syncs a changed applied range and links invalid dates to a focused error', async () => {
    const wrapper = mount(DesktopAnalyticsRangePicker, { props: { modelValue: { days: 30 } }, attachTo: document.body })
    await wrapper.setProps({ modelValue: { from: '2026-09-01', to: '2026-09-07' } })
    const dates = wrapper.findAll('input[type="date"]')
    expect((dates[0].element as HTMLInputElement).value).toBe('2026-09-01')
    await dates[1].setValue('2026-08-31')
    await wrapper.get('form').trigger('submit')
    expect(document.activeElement).toBe(dates[0].element)
    const alert = wrapper.get('[role="alert"]')
    expect(dates[0].attributes('aria-describedby')).toBe(alert.attributes('id'))
    expect(dates[0].attributes('aria-invalid')).toBe('true')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.setProps({ modelValue: { days: 7 } })
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.findAll('[role="group"] button')[0].attributes('aria-pressed')).toBe('true')
    wrapper.unmount()
  })

})
