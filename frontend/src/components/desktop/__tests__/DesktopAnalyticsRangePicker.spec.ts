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
})
