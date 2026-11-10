/**
 * Virtual root of the worker host's in-memory filesystem. Kept
 * in one module so the process shim, the path/os shims, and the VFS image
 * collector cannot drift apart.
 */

/** Virtual filesystem root; `process.cwd()` and every absolute path start here. */
export const GLH_ROOT = '/dsh'

/** `$GLH_HOME`: durable-state directory inside the image. */
export const GLH_HOME = `${GLH_ROOT}/home`

/** Flat, symlink-free package tree resolved by the worker module loader. */
export const GLH_NODE_MODULES = `${GLH_ROOT}/node_modules`

/** Image configuration directory: the VFS manifest and the shipped agent presets. */
export const GLH_CONFIG = `${GLH_ROOT}/config`

/** Default (empty) workspace directory. */
export const GLH_WORKSPACE = `${GLH_ROOT}/workspace`

/** Temporary directory reported by `os.tmpdir()`. */
export const GLH_TMP = `${GLH_ROOT}/tmp`
