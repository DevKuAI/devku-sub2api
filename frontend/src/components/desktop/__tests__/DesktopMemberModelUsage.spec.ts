import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { DesktopUsageMemberModel } from '@/api/desktopOrganizationUsage'
import Pagination from '@/components/common/Pagination.vue'
import DesktopMemberModelUsage from '../DesktopMemberModelUsage.vue'

vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))

const record = (index: number): DesktopUsageMemberModel => ({
  member_id: `mem_${index}`, name: `Member ${index}`, deleted: index === 0,
  model: index % 2 ? 'Model-B' : 'Model-A', requests: 2,
  input_tokens: 1234, output_tokens: 200, cache_creation_tokens: 300, cache_read_tokens: 400,
  total_tokens: 2134,
})

describe('Member model token usage', () => {
  it('groups models by member identity, totals their usage, and keeps same-name members separate', () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [record(0), { ...record(0), model: 'Model-C' }, { ...record(1), name: 'Member 0' }] } })
    const cards = wrapper.findAll('[data-testid="member-usage-card"]')
    expect(cards).toHaveLength(2)
    expect(cards[0].findAll('[data-testid="member-model-row"]')).toHaveLength(2)
    expect(cards[0].get('[data-testid="member-total"]').text()).toBe('4.27K')
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('6.4K')
    expect(wrapper.get('[data-testid="member-usage-count"]').text()).toBe('2')
    expect(cards[0].get('summary').text()).toContain('mem_0')
    expect(cards[1].get('summary').text()).toContain('mem_1')
    expect(cards[0].text()).toContain('conversations.deletedMember')
    expect(cards[1].text()).not.toContain('conversations.deletedMember')
    expect(cards[0].attributes('open')).toBeDefined()
    expect(cards[1].attributes('open')).toBeUndefined()
    wrapper.unmount()
  })

  it('uses compact units and lets every token category switch to precise counts', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [{ ...record(0), total_tokens: 2_345_678_901, input_tokens: 1_234_567, output_tokens: 999_995, cache_creation_tokens: 0, cache_read_tokens: 1_999_999_999 }] } })
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('2.35B')
    expect(wrapper.get('[data-token-kind="input_tokens"] dd').text()).toBe('1.23M')
    expect(wrapper.get('[data-token-kind="output_tokens"] dd').text()).toBe('1M')
    expect(wrapper.get('[data-token-kind="cache_creation_tokens"] dd').text()).toBe('0')
    await wrapper.get('[data-testid="member-token-exact"]').trigger('click')
    expect(wrapper.get('[data-testid="member-token-exact"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('2,345,678,901')
    expect(wrapper.get('[data-testid="model-total"]').text()).toBe('2,345,678,901Token')
    expect(wrapper.get('[data-token-kind="input_tokens"] dd').text()).toBe('1,234,567')
    expect(wrapper.get('[data-token-kind="output_tokens"] dd').text()).toBe('999,995')
    expect(wrapper.get('[data-token-kind="cache_read_tokens"] dd').text()).toBe('1,999,999,999')
    wrapper.unmount()
  })

  it('keeps the documented billion unit for very large counts', () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [{ ...record(0), total_tokens: 1_234_567_890_000 }] } })
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('1,234.57B')
    wrapper.unmount()
  })

  it('sorts members by tokens and models by tokens, with an optional name order', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [record(0), { ...record(1), total_tokens: 8000 }, { ...record(1), model: 'Model-C', total_tokens: 3000 }] } })
    expect(wrapper.findAll('summary')[0].text()).toContain('mem_1')
    expect(wrapper.findAll('[data-testid="member-model-row"]')[0].text()).toContain('Model-B')
    await wrapper.get('[data-testid="member-usage-sort"]').setValue('name')
    expect(wrapper.findAll('summary')[0].text()).toContain('mem_0')
    wrapper.unmount()
  })

  it('applies model and search filters to all overview totals and clears them', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [record(0), { ...record(0), model: 'Model-C', total_tokens: 1000 }, record(1)] } })
    await wrapper.get('[data-testid="member-model-filter"]').setValue('Model-C')
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('1K')
    expect(wrapper.get('[data-testid="member-usage-card"] summary').text()).toContain('mem_0')
    await wrapper.get('input[type="search"]').setValue('member 1')
    expect(wrapper.get('[data-testid="member-usage-empty"]').text()).toContain('noMatchingMemberModels')
    await wrapper.get('[data-testid="member-usage-empty"] button').trigger('click')
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="member-model-filter"]').element).toHaveProperty('value', '')
    expect(wrapper.get('input[type="search"]').element).toHaveProperty('value', '')
    wrapper.unmount()
  })

  it('paginates all members with their models intact, searches every page, and resets when data changes', async () => {
    const rows = Array.from({ length: 25 }, (_, index) => record(index))
    rows.push({ ...record(24), model: 'Model-C', total_tokens: 0 })
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows } })
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(10)
    wrapper.findComponent(Pagination).vm.$emit('update:page', 3)
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(5)
    expect(wrapper.text()).toContain('mem_24')
    await wrapper.get('input[type="search"]').setValue(' MEM_24 ')
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="member-model-row"]')).toHaveLength(2)
    expect(wrapper.findComponent(Pagination).exists()).toBe(false)
    await wrapper.get('input[type="search"]').setValue('')
    wrapper.findComponent(Pagination).vm.$emit('update:page', 3)
    await wrapper.vm.$nextTick()
    await wrapper.setProps({ rows: [record(3)] })
    expect(wrapper.findAll('[data-testid="member-usage-card"]')).toHaveLength(1)
    expect(wrapper.findComponent(Pagination).exists()).toBe(false)
    wrapper.unmount()
  })

  it('distinguishes loading from zero usage, offers refresh, and recovers from an unmatched search', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [], loading: true } })
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('—')
    expect(wrapper.find('[data-testid="member-usage-empty"]').exists()).toBe(false)
    expect(wrapper.get('input').attributes('disabled')).toBeDefined()
    expect(wrapper.attributes('aria-busy')).toBe('true')
    await wrapper.setProps({ loading: false })
    expect(wrapper.get('[data-testid="member-usage-total"]').text()).toBe('0')
    expect(wrapper.get('[data-testid="member-usage-empty"]').text()).toContain('noUsage')
    await wrapper.get('[data-testid="member-usage-empty"] button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    await wrapper.setProps({ rows: [record(0)] })
    await wrapper.get('input[type="search"]').setValue('unknown')
    expect(wrapper.get('[data-testid="member-usage-empty"]').text()).toContain('noMatchingMemberModels')
    expect(wrapper.get('[data-testid="member-usage-results"]').attributes('role')).toBe('status')
    wrapper.unmount()
  })
})
