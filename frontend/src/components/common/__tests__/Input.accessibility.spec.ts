import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import Input from '../Input.vue'

describe('Input accessible descriptions', () => {
  it('binds distinct labels when callers omit IDs', () => {
    const wrapper = mount(defineComponent({ render: () => h('form', [h(Input, { modelValue: '', label: 'Provider' }), h(Input, { modelValue: '', label: 'Model' })]) }))
    const inputs = wrapper.findAll('input').map(item => item.element)
    expect(inputs[0].id).not.toBe(inputs[1].id)
    expect(inputs.map(input => input.labels?.[0]?.textContent?.trim())).toEqual(['Provider', 'Model'])
    wrapper.unmount()
  })
  it('keeps the field identity and switches its description to the validation error', async () => {
    const wrapper = mount(Input, { props: { modelValue: '', label: 'Model', hint: 'Use an allowed model', maxlength: 200 } })
    const input = wrapper.get('input')
    const id = input.attributes('id')
    expect(input.element.labels?.[0]?.textContent?.trim()).toBe('Model')
    expect(wrapper.get(`[id="${input.attributes('aria-describedby')}"]`).text()).toBe('Use an allowed model')
    expect(input.attributes('maxlength')).toBe('200')
    await wrapper.setProps({ error: 'Choose a model' })
    expect(input.attributes('id')).toBe(id)
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.get(`[id="${input.attributes('aria-describedby')}"]`).text()).toBe('Choose a model')
    await wrapper.setProps({ error: '', id: 'configured-model' })
    expect(input.attributes('aria-invalid')).toBeUndefined()
    expect(wrapper.get('label').attributes('for')).toBe('configured-model')
    wrapper.unmount()
  })
})
