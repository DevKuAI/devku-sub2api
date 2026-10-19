import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import OrganizationDirectory from '../components/OrganizationDirectory.vue'
import RecordList from '../components/RecordList.vue'
import { i18n, organization, record } from './testing'

const api = vi.hoisted(() => ({ organizations: vi.fn(), summary: vi.fn(), listRecords: vi.fn(), exportCSV: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))
enableAutoUnmount(afterEach)
afterEach(() => vi.useRealTimers())

const DataTable = {
  name: 'DataTable', props: ['data', 'columns', 'loading'], emits: ['row-click'],
  template: '<table><tbody><tr v-for="row in data" :key="row.id" @click="$emit(\'row-click\', row)"><td v-for="column in columns" :key="column.key"><slot :name="`cell-${column.key}`" :row="row" /></td></tr><tr v-if="!loading && !data.length"><td><slot name="empty" /></td></tr></tbody></table>',
}
async function context() {
  const empty = { template: '<div />' }
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { name: 'AdminAccounting', path: '/admin/accounting', component: empty },
    { name: 'AdminAccountingReceivingAccounts', path: '/admin/accounting/accounts', component: empty },
    { name: 'AdminOrganizationAccounting', path: '/admin/accounting/organizations/:organizationId', component: empty },
    { name: 'AdminAccountingRecordNew', path: '/admin/accounting/organizations/:organizationId/:kind/new', component: empty },
    { name: 'AdminAccountingRecord', path: '/admin/accounting/organizations/:organizationId/:kind/:id', component: empty },
  ] })
  await router.push('/admin/accounting'); await router.isReady()
  return { router, global: { plugins: [i18n(), router], stubs: { DataTable, Icon: true, Pagination: true } } }
}

describe('accounting native navigation and recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizations.mockResolvedValue([organization])
    api.summary.mockResolvedValue([])
    api.listRecords.mockResolvedValue({ items: [record({ details: { payer_name: '客户付款' } })], total: 1 })
  })

  it('provides a real enterprise link and navigates once without bubbling into the row', async () => {
    const { router, global } = await context()
    const wrapper = mount(OrganizationDirectory, { global }); await flushPromises()
    const link = wrapper.get('tbody a')
    expect(link.attributes('href')).toBe('/admin/accounting/organizations/org-one')
    const push = vi.spyOn(router, 'push')
    link.element.addEventListener('click', event => { if ((event as MouseEvent).ctrlKey) event.preventDefault() })
    await link.trigger('click', { ctrlKey: true }); await flushPromises()
    expect(push).not.toHaveBeenCalled()
    await link.trigger('click'); await flushPromises()
    expect(push).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.name).toBe('AdminOrganizationAccounting')
  })

  it('provides a native admin record link without duplicate row navigation', async () => {
    const { router, global } = await context()
    const wrapper = mount(RecordList, { props: { kind: 'transfers', organizationId: 'org-one' }, global }); await flushPromises()
    const link = wrapper.get('tbody a')
    expect(link.attributes('href')).toBe('/admin/accounting/organizations/org-one/transfers/record-one')
    const push = vi.spyOn(router, 'push')
    await link.trigger('click'); await flushPromises()
    expect(push).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.params.id).toBe('record-one')
  })

  it('retries a failed directory request without reporting an empty enterprise directory', async () => {
    api.organizations.mockRejectedValueOnce({ code: 'ERR_NETWORK' })
    const { global } = await context()
    const wrapper = mount(OrganizationDirectory, { global }); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('检查网络后重试')
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('暂无企业')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('tbody a').text()).toBe(organization.name)
  })

  it('still renders a successful empty directory as empty data', async () => {
    api.organizations.mockResolvedValueOnce([])
    const { global } = await context()
    const wrapper = mount(OrganizationDirectory, { global }); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('暂无企业')
  })

  it('ignores an older directory request that finishes after a newer search', async () => {
    vi.useFakeTimers()
    let finish!: (rows: typeof organization[]) => void
    api.organizations.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const { global } = await context()
    const wrapper = mount(OrganizationDirectory, { global }); await flushPromises()
    api.organizations.mockResolvedValueOnce([{ ...organization, name: '最新搜索结果' }])
    await wrapper.get('input[type="search"]').setValue('new')
    await vi.advanceTimersByTimeAsync(300); await flushPromises()
    finish([organization]); await flushPromises()
    expect(wrapper.get('tbody a').text()).toBe('最新搜索结果')
  })
})
