# GLKJ Harness 构建、部署与打包指南

本文件面向开发、构建和运维人员，说明 GLKJ Harness 的环境要求、源码开发、生产 Web/CLI 部署、私有 npm 包族发布及 Python 运行时构建。日常 Web 使用和模型配置见 [USAGE.md](USAGE.md)。

## 1. 选择交付方式

| 场景 | 推荐方式 | 目标环境 | 启动入口 |
| --- | --- | --- | --- |
| 本地开发与联调 | 源码 checkout | Node.js、pnpm | `pnpm glh web` + `pnpm run dev:web` |
| 单机或内网生产 Web/CLI | 源码构建部署 | Node.js；构建机还需 pnpm | `node apps/cli/lib/bin.js web` |
| 团队规模化 CLI 分发 | 公司私有 npm Registry 的完整包族 | Node.js | `glh web` |
| SDK 或无人值守 agent | Python SDK + 平台 runtime wheel | Python ≥ 3.10 | Python 程序或 SDK 示例 |

> 当前 npm 包内部仍保留 `@deepseek-ai/*` 兼容命名空间。**公共 npm 的同名包是上游 DeepSeek 产品，不是本 GLKJ 分支。**不要使用 `npx @deepseek-ai/dsh web`，也不要只安装一个 CLI tarball 后直接运行；生产 npm 分发必须发布并消费同版本的完整包族，见第 7 节。

## 2. 环境要求

| 依赖 | 要求 | 用途 |
| --- | --- | --- |
| Node.js | `^22.19.0 || >=24.0.0` | CLI/Web 运行与构建 |
| pnpm | `11.7.0` | 源码依赖、构建与开发脚本 |
| Git | 较新版本 | 获取源码与版本管理 |
| Python（可选） | ≥ 3.10 | 使用 Python SDK |
| uv（可选） | 最新稳定版 | 构建或测试 Python wheel |
| Playwright Chromium（可选） | 仓库锁定版本 | 浏览器 e2e 测试 |

预留至少约 3 GB 用于源码和 Node 依赖。构建 Python 单文件运行时还需要约 1 GB 额外空间；单个 runtime executable 约 200 MB。

## 3. 获取源码、安装依赖与首次构建

```sh
git clone <GLKJ-repo-url>
cd deepseek-harness

# 构建/CI 推荐锁定依赖版本；正常安装会执行 postinstall，修复所需链接与权限。
pnpm install --frozen-lockfile
pnpm run build
```

`pnpm run build` 依次构建：

| 阶段 | 实际命令 | 主要产物 |
| --- | --- | --- |
| `build:lib:host` | `tsc -b tsconfig.host.json` + `tsdown --env.GLH_BUILD_FACE host` | Host 侧 workspace `lib/` |
| `build:lib:client` | `tsc -b tsconfig.client.json` + `tsdown --env.GLH_BUILD_FACE client` | Client plugin `lib/client.js` 与类型 |
| `build:web` | `pnpm --filter @deepseek-ai/dsh-web-frontend run build` | `apps/web/dist/` |

验收构件至少应存在：

```text
apps/cli/lib/bin.js
apps/web/dist/index.html
```

Web 前端由 GLKJ CLI 内嵌 HTTP 服务提供；**不要把独立 Vite 服务当作 GLKJ Web 产品服务器**，它没有运行时注入的宿主配置。

## 4. 开发环境启动

源码命令使用 TypeScript 入口，适用于开发和本地联调。全新 checkout 或产物已过期时先执行第 3 节的 `pnpm run build`。

```sh
# 终端 1：源码 Host + Web 服务
pnpm glh web

# 终端 2：仅监听 Web client-plugin 并重建 bundle
pnpm run dev:web
```

Web 服务默认在 <http://127.0.0.1:3080/> 提供页面。控制台打印 `glh web: ...` 才表示组合配置、`/api` 路由和前端 fallback 已完成加载。

### watcher 的作用与边界

- `pnpm run dev:web` 已固定通过 Node 运行 `scripts/dev-web.ts --poll`，不要手动改用 `tsx`。
- watcher 只发现启动时已有的 Web `dsh.client` 包，并重建它们的 `lib/client.js`；新增这类包后必须重启 watcher。
- Host、普通 Node 包、`apps/web` shell、Vite 配置及静态资源的变更不会自动通过该 watcher 生效；重新运行 `pnpm run build`、刷新浏览器，必要时重启 Web 服务。
- 两个进程必须从同一个 checkout 启动。按 `Ctrl+C` 分别停止；监督程序使用 `SIGTERM` 可请求优雅关闭。

## 5. 生产 Web/CLI 部署

### 5.1 安全与状态目录

GLKJ Web 默认只绑定 `127.0.0.1`，当前会拒绝 `--host 0.0.0.0`。如需远程访问，请使用经过认证和访问控制的反向代理或 SSH 隧道，保持 GLKJ 仅在 loopback 监听。

为生产服务账户设置独立且受权限保护的状态目录：

```sh
export GLH_HOME=/var/lib/glkj-harness
export GLKJ_API_KEY="<模型服务密钥>"
export GLKJ_BASE_URL="https://<公司模型网关>/v1"
```

`GLH_HOME` 默认是 `~/.glh`；其中 `.credentials.yaml` 存储 Web UI 凭据，`settings.yaml` 存储模型与供应商配置。不要把这些文件写进 Git、镜像层或日志；按公司密钥管理和备份策略保护它们。

默认 `GLKJ_BASE_URL` 为 `https://api.glkj.example.com`，这只是占位地址，不能用于正式调用。

### 5.2 使用构建后的生产入口启动

从已构建的源码目录启动：

```sh
cd /opt/glkj/deepseek-harness
node apps/cli/lib/bin.js web
```

常用参数：

```sh
# 默认：127.0.0.1:3080
node apps/cli/lib/bin.js web

# 指定固定端口
node apps/cli/lib/bin.js web --port 8080

# 由系统分配一个空闲端口
node apps/cli/lib/bin.js web --port 0

# 检查组合配置，不启动服务
node apps/cli/lib/bin.js web --dump-config

# 无界面执行一次任务
node apps/cli/lib/bin.js --profile headless "任务描述"
```

启动成功时进程输出 `glh web: http://127.0.0.1:<port>`。可将该输出、`GET /` 返回 200 和一次受控 `POST /api/host.describe` 作为进程 readiness 检查。

### 5.3 首次模型配置

GLKJ 不出厂预选模型。部署环境设置 API Key/Base URL 或在 UI 保存供应商信息后，仍需在 **Settings → Models** 中配置供应商并在会话模型选择器中选中一个模型，才能发送消息。

如果出现 `404 status code (no body)`，通常说明模型选择已成功、但上游模型服务拒绝了真实生成请求。检查供应商协议、Base URL 与路径是否匹配：例如 `/v1` 重复或遗漏，或将 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 协议配错。它不是本地 Web 服务的默认模型问题。

## 6. 构建验证与测试

按需要选择：

```sh
pnpm run build                         # 构建 Host/Client/Web
pnpm test                              # 全量 Vitest
pnpm run test:e2e                      # 真实 API e2e；无密钥时相关测试跳过
pnpm run test:snapshot                 # Web 快照回放
pnpm run test:web                      # 构建后浏览器 e2e
pnpm run verify-translation-pairing    # 翻译配对记录
pnpm run doc-sync                      # 文档一致性门禁
```

浏览器 e2e 需要安装 Playwright Chromium：

```sh
node node_modules/.pnpm/playwright@*/node_modules/playwright/cli.js install chromium --only-shell
```

Linux 缺少 `libnspr4.so`、`libnss3.so` 等 Chromium 系统库时，应由系统管理员安装相应依赖后再运行浏览器 e2e；这与 GLKJ 应用构建本身无关。

## 7. npm 打包与私有 Registry 发布

### 7.1 单个 CLI tarball 的边界

以下命令会生成 CLI tarball：

```sh
pnpm run build
pnpm --filter @deepseek-ai/dsh pack
```

产物通常位于仓库根目录，例如 `deepseek-ai-dsh-<version>.tgz`。它仅包含 CLI 本身的构建入口和配置，不是能从公共 npm 独立安装的 GLKJ 完整产品。由于内部兼容包名与上游重叠，单包安装会把依赖解析到公共 npm 的上游版本，导致运行时导出不匹配。

### 7.2 团队私有 Registry 的完整发布

仅由发布管理员在受控环境执行。完整交付需要将同一版本的 dsh family、vendor 依赖和 Landlock entry 一起打包、验证并发布；不要只上传 CLI tarball。

```sh
pnpm install --frozen-lockfile
pnpm run release:verify --family dsh
pnpm run build
pnpm run release:pack --family dsh --out dist/npm
pnpm run release:pack --family vendor --out dist/npm-vendor

# 还需要纳入由正式发布流程构建的 dist/npm-landlock。
pnpm run release:verify-packed-install \
  --family dsh \
  --from dist/npm \
  --from dist/npm-vendor \
  --from dist/npm-landlock
```

通过验证的 tarball 输出于 `dist/npm/` 等目录。完成公司的版本、tag、Registry 权限和发布审批流程后，才可执行：

```sh
pnpm run release:publish --family dsh --from dist/npm
```

消费端必须配置公司的私有 Registry，再安装完整包族提供的 `@deepseek-ai/dsh` 并以 `glh web` 启动。不要把发布凭据、Registry token 或 API Key 写进文档、shell 历史、仓库或构建日志。

## 8. Python SDK 与单文件运行时（可选）

Python SDK 适合 SDK 集成和无人值守 agent。正式 runtime target 是：

- `node24-linux-x64`
- `node24-linux-arm64`
- `node24-macos-arm64`

Python agent runtime 当前不支持 Windows。SDK 用户需要 Python ≥ 3.10，安装匹配的 runtime wheel 后不需要系统 Node。

构建 Linux x64 runtime：

```sh
pnpm exec tsx scripts/build-exe-for-python-sdk.ts \
  --targets=node24-linux-x64
```

产物为：

```text
dist-exe/dsh-jsonrpc-agent-pkg-linux-x64
```

脚本会构建项目、准备 runtime dependency closure，并同步 executable 到 Python runtime 包。构建 wheel 的正式步骤与平台矩阵见 `python/development.zh.md` 和 Python release workflow；不要用旧的 `node22-*` target 示例。

SDK 使用者参考 `python/sdk/README.zh.md`。运行的 agent 默认能够执行 shell 并写入 workspace，必须使用隔离 workspace 或容器并限制凭据权限。

## 9. 排障速查

| 现象 | 处理 |
| --- | --- |
| `frontend dist not built` | 在仓库根目录运行 `pnpm run build`。 |
| 编辑器提示尚未选择模型 | 先配置供应商，然后在会话中显式选择模型。 |
| 上游返回 `404 status code (no body)` | 修正自定义模型供应商的协议/Base URL/路径；不是本地 `/api` 路由 404。 |
| 本地 3080 空响应 404 | 等待 `glh web:` 就绪行后再访问，确认没有连接到其他端口或已退出的进程。 |
| 端口冲突 | 用 `node apps/cli/lib/bin.js web --port <port>` 或开发模式中的 `pnpm glh web --port <port>`。 |
| 单 tarball 全局安装失败 | 改用源码构建部署，或发布完整同版本包族至私有 Registry。 |
| 浏览器 e2e 无法启动 | 安装 Playwright Chromium，并让系统管理员补齐所需共享库。 |

## 10. 命令速查

```sh
# 源码安装与构建
pnpm install --frozen-lockfile
pnpm run build

# 开发环境（两个终端）
pnpm glh web
pnpm run dev:web

# 构建后的生产入口
node apps/cli/lib/bin.js web

# 生产打包管理员：完整 dsh package family
pnpm run release:pack --family dsh --out dist/npm
```
