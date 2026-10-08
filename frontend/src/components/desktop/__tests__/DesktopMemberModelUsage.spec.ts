import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { DesktopUsageMemberModel } from '@/api/desktopOrganizationUsage'
import Pagination from '@/components/common/Pagination.vue'
import DesktopMemberModelUsage from '../DesktopMemberModelUsage.vue'

vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const record = (index: number): DesktopUsageMemberModel => ({
  member_id: `mem_${index}`, name: `Member ${index}`, deleted: index === 0,
  model: index % 2 ? 'Model-B' : 'Model-A', requests: 2,
  input_tokens: 1234, output_tokens: 200, cache_creation_tokens: 300, cache_read_tokens: 400,
  total_tokens: 2134,
})

describe('Member model token usage', () => {
  it('shows all token categories with exact counts and distinguishes same-name members', () => {
    const rows = [record(0), { ...record(1), name: 'Member 0' }]
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows } })
    const rendered = wrapper.findAll('tbody tr')
    expect(rendered).toHaveLength(2)
    expect(rendered[0].text()).toContain('conversations.deletedMember')
    expect(rendered[1].text()).not.toContain('conversations.deletedMember')
    expect(rendered[0].text()).toContain('mem_0')
    expect(rendered[1].text()).toContain('mem_1')
    expect(rendered[0].findAll('td').map(cell => cell.text()).slice(2)).toEqual([
      '2', (1234).toLocaleString(), '200', '300', '400', (2134).toLocaleString(),
    ])
    wrapper.unmount()
  })

  it('paginates beyond the top ten, searches all rows, and resets the page when data changes', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: Array.from({ length: 25 }, (_, index) => record(index)) } })
    expect(wrapper.findAll('tbody tr')).toHaveLength(20)
    wrapper.findComponent(Pagination).vm.$emit('update:page', 2)
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('tbody tr')).toHaveLength(5)
    expect(wrapper.text()).toContain('mem_24')
    await wrapper.find('input[type="search"]').setValue(' MEM_0 ')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).toContain('mem_0')
    expect(wrapper.findComponent(Pagination).props('page')).toBe(1)
    await wrapper.find('input[type="search"]').setValue('model-b')
    expect(wrapper.findAll('tbody tr')).toHaveLength(12)
    await wrapper.find('input[type="search"]').setValue('member 24')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    await wrapper.find('input[type="search"]').setValue('')
    wrapper.findComponent(Pagination).vm.$emit('update:page', 2)
    await wrapper.vm.$nextTick()
    await wrapper.setProps({ rows: [record(3)] })
    expect(wrapper.findComponent(Pagination).props('page')).toBe(1)
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    wrapper.unmount()
  })

  it('distinguishes no usage from an unmatched search', async () => {
    const wrapper = mount(DesktopMemberModelUsage, { props: { rows: [] } })
    expect(wrapper.text()).toContain('usageStatistics.noUsage')
    expect(wrapper.findComponent(Pagination).exists()).toBe(false)
    await wrapper.setProps({ rows: [record(0)] })
    await wrapper.find('input[type="search"]').setValue('unknown')
    expect(wrapper.text()).toContain('usageStatistics.noMatchingMemberModels')
    expect(wrapper.findComponent(Pagination).exists()).toBe(false)
    wrapper.unmount()
  })
})
