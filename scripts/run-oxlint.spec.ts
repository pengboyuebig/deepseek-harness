import { describe, expect, it } from 'vitest'
import { resolveOxlintInvocation } from './run-oxlint.ts'

describe('Oxlint invocation', () => {
  it('preserves the ordinary default invocation', () => {
    expect(resolveOxlintInvocation(['.'], { PATH: '/bin' })).toEqual({
      args: ['.'],
      env: { PATH: '/bin' },
    })
  })

  it('bounds both worker pools from one setting', () => {
    expect(resolveOxlintInvocation(['.', '--fix'], { GLH_OXLINT_THREADS: '4', GOMAXPROCS: '12' })).toEqual({
      args: ['.', '--fix', '--threads=4'],
      env: { GLH_OXLINT_THREADS: '4', GOMAXPROCS: '4' },
    })
  })

  it.each(['0', '-1', '1.5', 'auto'])('rejects invalid worker bound %s', (value) => {
    expect(() => resolveOxlintInvocation(['.'], { GLH_OXLINT_THREADS: value }))
      .toThrow('GLH_OXLINT_THREADS must be a positive integer')
  })

  it('rejects a competing direct worker bound', () => {
    expect(() => resolveOxlintInvocation(['.', '--threads=2'], { GLH_OXLINT_THREADS: '4' }))
      .toThrow('use GLH_OXLINT_THREADS instead')
  })
})
