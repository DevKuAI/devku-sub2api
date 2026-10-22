import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { desktopStructure, OrganizationUnit } from '@/api/desktopStructure'
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import { useDesktopStructureOptions } from './useDesktopStructureOptions'

describe('Desktop structure options', () => {
  it('cancels replaced reads and ignores their delayed results', async () => {
    let options!: ReturnType<typeof useDesktopStructureOptions>
    const wrapper = mount(defineComponent({ setup() { options = useDesktopStructureOptions(); return () => h('div') } }))
    let finish!: (value: OrganizationUnit[]) => void
    const all = vi.fn().mockReturnValueOnce(new Promise(resolve => { finish = resolve })).mockResolvedValue([])
    const api = { all } as unknown as ReturnType<typeof desktopStructure>
    const old = options.load(api, { positions: false })
    const oldSignal = all.mock.calls[0]![2] as AbortSignal
    await options.load(api, { positions: false })
    expect(oldSignal.aborted).toBe(true)
    finish([{ public_id: 'dep_old' } as OrganizationUnit]); await old
    expect(options.departments.value).toEqual([])
    wrapper.unmount()
    expect((all.mock.calls[1]![2] as AbortSignal).aborted).toBe(true)
  })
  it('aborts sibling reads on failure and retries without exposing stale options after reset', async () => {
    let options!: ReturnType<typeof useDesktopStructureOptions>
    const wrapper = mount(defineComponent({ setup() { options = useDesktopStructureOptions(); return () => h('div') } }))
    const all = vi.fn().mockRejectedValueOnce(new Error('Unavailable')).mockResolvedValue([])
    const api = { all } as unknown as ReturnType<typeof desktopStructure>
    await options.load(api)
    expect(options.error.value).toBe('Unavailable')
    expect((all.mock.calls[1]![2] as AbortSignal).aborted).toBe(true)
    await options.load(api)
    expect(options.error.value).toBe('')
    options.reset()
    expect(options.loading.value).toBe(false)
    expect(options.departments.value).toEqual([])
    wrapper.unmount()
  })
})
