import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import Pagination from '../Pagination.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('Pagination accessible names', () => {
  it('uses localized labels for the navigation and page-size selector', () => {
    const wrapper = mount(Pagination, {
      props: { page: 1, pageSize: 20, total: 40 },
      global: { stubs: { Icon: true } },
    })

    expect(wrapper.get('nav').attributes('aria-label')).toBe('pagination.label')
    expect(wrapper.get('.page-size-select .select-trigger').attributes('aria-label')).toBe('pagination.perPage')
  })
})
