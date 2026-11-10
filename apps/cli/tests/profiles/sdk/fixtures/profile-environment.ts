import { join } from 'node:path'
import { clearAmbientProxyEnv } from '../../../../../../scripts/test-proxy-environment.ts'

/** Isolate SDK profile processes from ambient profile, runtime and Node loader settings. */
export function profileEnvironment(root: string, baseUrl: string): NodeJS.ProcessEnv {
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('GLH_')))
  Object.assign(env, {
    GLH_PERMISSION_MODE: 'danger-full-access', GLH_TELEMETRY_DISABLED: '1',
    GLH_AGENTS_HOME: join(root, 'agents'), GLH_PRIMARY_RUNTIME: '',
    GLKJ_API_KEY: 'keyless-profile-test', GLKJ_BASE_URL: baseUrl,
    NODE_OPTIONS: '', NODE_PATH: '',
  })
  clearAmbientProxyEnv(env)
  return env
}
