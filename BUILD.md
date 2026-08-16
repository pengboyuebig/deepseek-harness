# GLKJ Harness 构建与打包指南（BUILD.md）

面向部署/运维：从零环境到启动、构建、打包、分发的完整步骤。
日常使用见 [USAGE.md](USAGE.md)；品牌占位素材替换见 [BRANDING.md](BRANDING.md)。

## 1. 环境要求

| 依赖 | 版本要求 | 用途 | 安装方式 |
| --- | --- | --- | --- |
| Node.js | **≥ 22 LTS**（构建机推荐 22.x；上游 CI 打包 exe 用 24.x） | 运行时与构建 | [nodejs.org](https://nodejs.org) 或系统包管理器 |
| pnpm | **≥ 11.7** | monorepo 依赖与脚本 | `npm install -g pnpm@11` |
| Git | 任意较新版本 | 拉取仓库 | — |
| Python + uv（可选） | Python ≥ 3.12 + [uv](https://docs.astral.sh/uv/) | 仅构建/使用 Python SDK 时需要 | `curl -LsSf https://astral.sh/uv/install.sh \| sh` |
| Playwright Chromium（可选） | 与仓库锁定的 1.61.x | 仅跑 Web 浏览器 e2e 测试时需要 | 见第 7 节 |

磁盘要求：源码 + 依赖约 3 GB；若构建 Python 单文件运行时（exe），额外约 1 GB（产物单文件约 200 MB）。

## 2. 获取源码与安装依赖

```sh
git clone <GLKJ-repo-url>
cd deepseek-harness

# 安装全部 workspace 依赖（会自动执行 postinstall：lefthook 钩子、
# node-pty 补丁、spawn-helper 权限修复等）
pnpm install
```

离线/内网环境可加 `--prefer-offline`。国内网络拉 npm 包慢时，可先设置镜像：

```sh
pnpm config set registry https://registry.npmmirror.com
```

## 3. 开发模式启动（无需构建）

源码经 tsx 直跑，改代码即时生效：

```sh
pnpm glh web                      # Web UI，默认 http://127.0.0.1:3080
pnpm glh --profile tui            # 终端 UI
pnpm glh --profile headless "任务"  # 单任务无界面
```

Web 前端热更新开发：`pnpm run dev:web`（配合 `pnpm glh web` 使用）。

启动前需要配置并选择模型。GLKJ 不预设默认模型，因此首次进入 Web UI 后，打开 Models 页配置供应商、Base URL、API Key 和模型，再在会话模型选择器中选中该模型；未选择前不会向占位地址请求：

```sh
export GLKJ_API_KEY="你的密钥"
export GLKJ_BASE_URL="https://你的网关地址"   # 不设则为占位地址 api.glkj.example.com
```

## 4. 构建产物（pnpm run build）

```sh
pnpm run build
```

等价于两步：

| 步骤 | 命令 | 产物 |
| --- | --- | --- |
| build:lib | `tsc -b` + `tsdown`（host/client 两个 face） | 每个 `packages/*/*/lib/` 下的 JS + d.ts |
| build:web | Vite 构建 `apps/web` 前端 | `apps/web/dist/`（由 CLI 在运行时伺服） |

构建后即可用**纯 Node 运行**（不再依赖 tsx/TS 源码）：

```sh
node apps/cli/lib/bin.js web        # 与 pnpm glh web 等价，但走构建产物
```

## 5. 打包为可安装的 npm 包并分发

CLI 包名 `@deepseek-ai/dsh`，bin 命令为 `glh`，打包内容为 `lib/*.js` 与 `config/`。

```sh
# 1. 先构建（打包不自动构建）
pnpm run build

# 2. 在 workspace 内打出 tarball（产物生成在仓库根目录）
pnpm --filter @deepseek-ai/dsh pack

# 产物示例：./deepseek-ai-dsh-0.1.0-rc.5.tgz（内含 lib/*.js 与 config/）
```

### ⚠️ 重要：不要直接 `npm install -g <tarball>`

本分支保留了上游包名 `@deepseek-ai/dsh-*`（品牌只改用户可见层，见
BRANDING.md「有意保留」）。npm 公共仓库上的同名依赖是**上游 DeepSeek 版本**
（旧导出名），tarball 安装时会从 registry 拉上游依赖，启动即报
`does not provide an export named 'GLH_...'` 崩溃——已实测验证。

### 正确的分发方式（三选一）

**方式 A：随源码仓库分发（最简单，已验证可用）**

目标机器克隆本仓库并 `pnpm install && pnpm run build` 后，直接运行构建产物：

```sh
node apps/cli/lib/bin.js web            # 等价于 pnpm glh web，但走构建产物
node apps/cli/lib/bin.js --profile headless "任务描述"
```

**方式 B：私有 registry 发布全家桶（团队规模化推荐）**

把 workspace 内全部 `@deepseek-ai/dsh-*` 包（版本号需整体高于上游，例如
`0.1.0-rc.5` → 用私有版本段）发布到 Verdaccio/Nexus，团队机器配置：

```sh
npm config set registry https://你的私服地址/
npm install -g @deepseek-ai/dsh
glh web
```

**方式 C：Python 单文件运行时**（完全无 Node 依赖，见第 6 节），适合
只跑无人值守 agent 的场景。

## 6. Python SDK 运行时打包（可选）

Python SDK 依赖一个单文件 Node 运行时（内置全部插件闭包）：

```sh
# 构建 linux-x64 单文件运行时（约 200 MB）
# 目标格式：<nodeRange>-<platform>-<arch>，如 node22-linux-x64 / node24-macos-arm64
npx tsx scripts/build-exe-for-python-sdk.ts --targets=node22-linux-x64

# 产物：dist-exe/dsh-jsonrpc-agent-pkg-linux-x64
# 同时同步到 python/sdk-runtime/src/deepseek_harness_runtime/runtime/
```

再打 Python wheel：

```sh
# 生成 deepseek_harness_sdk-<版本>-py3-none-any.whl（需要 uv）
python3 scripts/build-python-release.py --runtime-exe dist-exe/dsh-jsonrpc-agent-pkg-linux-x64
```

使用方安装 wheel 后（详见 USAGE.md 第 6 节）：

```python
from deepseek_harness import DeepSeekHarnessClient, HarnessConfig
client = DeepSeekHarnessClient(HarnessConfig(base_url="...", api_key="..."))
```

本地验证 Python 侧：

```sh
cd python/sdk && uv run --with pytest pytest -q
```

## 7. 测试环境补充（可选）

```sh
pnpm test                    # 全量 vitest（无需额外环境）
pnpm run test:e2e            # 真实 API e2e：需 GLKJ_API_KEY，无密钥自动跳过
pnpm run test:snapshot       # Web UI 快照回放（无需浏览器）
pnpm run test:web            # 浏览器 e2e：需先装 Playwright Chromium
```

Playwright 浏览器安装：

```sh
node node_modules/.pnpm/playwright@*/node_modules/playwright/cli.js install chromium --only-shell
```

Linux 下若报缺共享库（如 `libnspr4.so`、`libnss3`），需系统管理员安装：
`sudo apt install libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2`

## 8. 常见问题

| 现象 | 处理 |
| --- | --- |
| `tsc` 报找不到 `scripts/oxlint-contract-*.ts` | 删除根目录 `*.tsbuildinfo` 后重试（陈旧增量缓存） |
| 启动后模型请求失败/404 | `GLKJ_BASE_URL` 仍是占位地址，改为公司网关（BRANDING.md 1.4） |
| `pnpm install` 后 subprocess 测试失败 | 重跑 `pnpm install` 恢复 node-pty 补丁与包链接 |
| exe 运行时报 `DSH_CORDIS_CONFIG` 之类的旧变量名 | 运行时二进制是改造前构建的，重跑第 6 节的构建命令 |
| 浏览器 e2e 全部失败 | 未安装 Playwright Chromium 或缺系统库（第 7 节） |

## 9. 速查

```sh
pnpm install                          # ① 装依赖
pnpm glh web                          # ② 开发直跑
pnpm run build                        # ③ 构建
node apps/cli/lib/bin.js web          # ④ 构建产物直跑（已验证：HTTP 200，标题 GLKJ Harness）
pnpm --filter @deepseek-ai/dsh pack   # ⑤ 打 npm tarball（分发方式见第 5 节警告）
```
