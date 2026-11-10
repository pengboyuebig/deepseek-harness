import { mkdir, mkdtemp, realpath, rm, symlink, writeFile } from 'node:fs/promises'
import { homedir, tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  AGENTS_HOME_DIR_NAME,
  DEFAULT_AGENTS_HOME_DISPLAY,
  DEFAULT_DSH_HOME_DISPLAY,
  GLH_HOME_DIR_NAME,
  agentsHomeDisplay,
  canonicalizeWatchPath,
  defaultAgentsHome,
  defaultDshHome,
  dshCachePath,
  dshHomeDisplay,
  dshHomePath,
  expandHomePath,
  resolveAgentsHome,
  resolveDshHome,
} from '@deepseek-ai/dsh-home-paths'

afterEach(() => {
  vi.unstubAllEnvs()
})

describe('dsh path helpers', () => {
  it('owns the shared default DSH home directory name', () => {
    expect(GLH_HOME_DIR_NAME).toBe('.glh')
    expect(DEFAULT_DSH_HOME_DISPLAY).toBe('~/.glh')
    expect(defaultDshHome()).toBe(join(homedir(), '.glh'))
  })

  it('expands tilde paths without changing non-tilde paths', () => {
    expect(expandHomePath('~')).toBe(homedir())
    expect(expandHomePath('~/.glh')).toBe(join(homedir(), '.glh'))
    expect(expandHomePath('~\\.glh')).toBe(join(homedir(), '.glh'))
    expect(expandHomePath('/tmp/.dsh')).toBe('/tmp/.dsh')
    expect(expandHomePath('~other/.dsh')).toBe('~other/.dsh')
  })

  it('resolves explicit path before GLH_HOME and the default', () => {
    const envHome = join(homedir(), 'env-dsh')

    expect(resolveDshHome('/tmp/explicit-dsh', { GLH_HOME: '~/env-dsh' })).toBe(resolve('/tmp/explicit-dsh'))
    expect(resolveDshHome(undefined, { GLH_HOME: '~/env-dsh' })).toBe(envHome)
    expect(resolveDshHome(undefined, {})).toBe(defaultDshHome())
  })

  it('treats an empty or whitespace-only GLH_HOME as unset', () => {
    expect(resolveDshHome(undefined, { GLH_HOME: '' })).toBe(defaultDshHome())
    expect(resolveDshHome(undefined, { GLH_HOME: '   ' })).toBe(defaultDshHome())
  })

  it('joins child segments onto the resolved GLH_HOME', () => {
    vi.stubEnv('GLH_HOME', '~/env-dsh')
    expect(dshHomePath()).toBe(join(homedir(), 'env-dsh'))
    expect(dshHomePath('storages', 'cache')).toBe(join(homedir(), 'env-dsh', 'storages', 'cache'))
  })

  it('labels a resolved home by whether it is the default root', () => {
    expect(dshHomeDisplay(resolve(defaultDshHome()))).toBe('~/.glh')
    expect(dshHomeDisplay('/some/other/root')).toBe('$GLH_HOME')
  })

  it('owns the shared default agents home directory name', () => {
    expect(AGENTS_HOME_DIR_NAME).toBe('.agents')
    expect(DEFAULT_AGENTS_HOME_DISPLAY).toBe('~/.agents')
    expect(defaultAgentsHome()).toBe(join(homedir(), '.agents'))
  })

  it('resolves an explicit agents home before GLH_AGENTS_HOME and the default', () => {
    const envHome = join(homedir(), 'env-agents')

    expect(resolveAgentsHome('/tmp/explicit-agents', { GLH_AGENTS_HOME: '~/env-agents' })).toBe(resolve('/tmp/explicit-agents'))
    expect(resolveAgentsHome(undefined, { GLH_AGENTS_HOME: '~/env-agents' })).toBe(envHome)
    expect(resolveAgentsHome(undefined, {})).toBe(defaultAgentsHome())
  })

  it('treats an empty or whitespace-only GLH_AGENTS_HOME as unset', () => {
    expect(resolveAgentsHome(undefined, { GLH_AGENTS_HOME: '' })).toBe(defaultAgentsHome())
    expect(resolveAgentsHome(undefined, { GLH_AGENTS_HOME: '   ' })).toBe(defaultAgentsHome())
  })

  it('labels a resolved agents home by whether it is the default root', () => {
    expect(agentsHomeDisplay(resolve(defaultAgentsHome()))).toBe('~/.agents')
    expect(agentsHomeDisplay('/some/other/root')).toBe('$GLH_AGENTS_HOME')
  })

  it.each([
    [undefined, join(homedir(), '.glh')],
    ['', join(homedir(), '.glh')],
    ['   ', join(homedir(), '.glh')],
    ['~/env-dsh', join(homedir(), 'env-dsh')],
    ['./relative-dsh', resolve('./relative-dsh')],
  ] as const)('resolves cache paths with GLH_HOME=%j', (home, expectedHome) => {
    vi.stubEnv('GLH_HOME', home)
    try {
      expect(dshCachePath()).toBe(join(expectedHome, 'cache'))
      expect(dshCachePath('models', 'index.json')).toBe(join(expectedHome, 'cache', 'models', 'index.json'))
    } finally {
      vi.unstubAllEnvs()
    }
  })

  it('resolves configured cache homes before the environment', () => {
    vi.stubEnv('GLH_HOME', '~/env-dsh')
    try {
      expect(dshCachePath({ dshHome: '~/explicit-dsh' })).toBe(join(homedir(), 'explicit-dsh', 'cache'))
      expect(dshCachePath({ dshHome: './explicit-dsh' }, 'attachments', 'request-images'))
        .toBe(resolve('./explicit-dsh/cache/attachments/request-images'))
      expect(dshCachePath({}, 'attachments')).toBe(join(homedir(), 'env-dsh', 'cache', 'attachments'))
    } finally {
      vi.unstubAllEnvs()
    }
  })

  it('canonicalizes a watcher ancestor while preserving a missing suffix', async () => {
    const root = await mkdtemp(join(tmpdir(), 'dsh-watch-path-'))
    const target = join(root, 'target')
    const alias = join(root, 'alias')
    try {
      await mkdir(target)
      await symlink(target, alias, process.platform === 'win32' ? 'junction' : 'dir')
      await expect(canonicalizeWatchPath(alias)).resolves.toBe(await realpath(target))
      await expect(canonicalizeWatchPath(join(alias, 'later', 'config.yml'))).resolves.toBe(
        join(await realpath(target), 'later', 'config.yml'),
      )
      const file = join(root, 'file')
      await writeFile(file, 'not a directory')
      await expect(canonicalizeWatchPath(join(file, 'child'))).rejects.toMatchObject({ code: 'ENOTDIR' })
    } finally {
      await rm(root, { recursive: true, force: true })
    }
  })
})
