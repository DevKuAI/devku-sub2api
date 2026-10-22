import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, reactive } from 'vue'

const navigation = vi.hoisted(() => ({ push: vi.fn(), guards: [] as Array<() => boolean> }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/members/new' }), useRouter: () => navigation,
  onBeforeRouteLeave: (guard: () => boolean) => navigation.guards.push(guard), onBeforeRouteUpdate: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 42 } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
import { useDesktopForm } from './useDesktopForm'

function form(organization: string) {
  const state = reactive({ name: '' })
  let guard!: ReturnType<typeof useDesktopForm>
  const wrapper = mount(defineComponent({ setup() {
    guard = useDesktopForm(() => state, () => false, () => organization)
    guard.clean()
    return () => h('div')
  } }))
  return { state, guard, wrapper }
}

describe('Desktop form draft lifecycle', () => {
  beforeEach(() => {
    navigation.guards.length = 0; navigation.push.mockReset().mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })
  it('consumes a child-navigation draft so reopening after a discard starts fresh', async () => {
    const parent = form('org_consume'); parent.state.name = 'Earlier draft'
    await parent.guard.child('/positions/new'); parent.wrapper.unmount()
    const returned = form('org_consume')
    Object.assign(returned.state, returned.guard.restore())
    expect(returned.state.name).toBe('Earlier draft')
    returned.state.name = 'Later edits'
    expect(navigation.guards.at(-1)!()).toBe(true)
    returned.wrapper.unmount()
    const reopened = form('org_consume')
    expect(reopened.guard.restore()).toBeUndefined()
    reopened.wrapper.unmount()
  })
  it('clears a cached draft on an explicit route discard but preserves it when leaving is rejected', async () => {
    const parent = form('org_discard'); parent.state.name = 'Draft'
    await parent.guard.child('/positions/new')
    vi.mocked(window.confirm).mockReturnValueOnce(false)
    expect(navigation.guards[0]!()).toBe(false)
    expect(parent.guard.restore()).toEqual({ name: 'Draft' })
    await parent.guard.child('/positions/new')
    vi.mocked(window.confirm).mockReturnValueOnce(true)
    expect(navigation.guards[0]!()).toBe(true)
    expect(parent.guard.restore()).toBeUndefined()
    parent.wrapper.unmount()
  })
})
