# GLKJ Harness 使用方法

GLKJ Harness（命令 `glh`）是基于 DeepSeek Harness 定制的插件化智能体框架。
本文面向使用方，覆盖安装、启动、模型接入、配置与 Python SDK；品牌占位素材的替换见 [BRANDING.md](BRANDING.md)。

## 1. 前置条件

- Node.js ≥ 22（推荐 22 LTS）
- pnpm ≥ 11（仅源码运行需要）
- 可访问的模型 API 端点与密钥（见第 4 节）

## 2. 快速开始

### 从仓库运行（推荐）

> 注意：不要 `npx @deepseek-ai/dsh web`——npm 公共仓库上该包名是上游
> DeepSeek 版本（旧品牌），本分支的获取方式见下，详见 [BUILD.md](BUILD.md)。

```sh
git clone <GLKJ-repo-url>
cd deepseek-harness
pnpm install
pnpm run build
pnpm glh web
```

启动后浏览器访问 `http://127.0.0.1:3080` 进入 Web UI（开发调试可跳过
`pnpm run build`，`pnpm glh web` 直跑源码）。

## 3. 常用命令

| 命令 | 作用 |
| --- | --- |
| `glh web` | 启动 Web UI（等价 `glh --profile web`），默认 `127.0.0.1:3080` |
| `glh --profile tui` | 启动终端 UI |
| `glh --profile headless "任务描述"` | 无界面跑单个任务，打印最终答复后退出 |
| `glh --profile <p> --patch ./overlay.yml` | 在指定 profile 上叠加自定义补丁层 |
| `glh web --dump-config` | 打印组合后的配置树，不启动（排障用） |
| `glh plugin --profile tui add <pkg>` | 向 profile 安装插件（转发给 pnpm） |
| `glh -V` | 查看版本 |

命令行帮助：`glh -h`（各 profile 自己的旗标用 `glh --profile <name> --help` 查看）。

## 4. 模型接入（后端配置）

默认端点当前为**占位地址** `https://api.glkj.example.com`，接入公司网关前需配置真实地址。

两种方式（任选）：

**方式一：环境变量**（对 LLM 与搜索均生效）

```sh
export GLKJ_API_KEY="你的密钥"
export GLKJ_BASE_URL="https://你的网关地址"   # 不设则用默认端点
glh web
```

也可写入分层 `.env`：项目目录或 `~/.glh/.env`（`GLKJ_API_KEY` 这类引导变量只能
export，不能进 `.env`，启动器会拒绝并提示）。

**方式二：Web UI 配置页** — 首次进入 Web UI 会弹出引导对话框，在 Models 页填写
Base URL 与 API Key，密钥存储在 `~/.glh/.credentials.yaml`，轮换后无需重启。

### 主要环境变量

| 变量 | 说明 |
| --- | --- |
| `GLKJ_API_KEY` | 模型 API 密钥 |
| `GLKJ_BASE_URL` | 模型端点（默认占位 `https://api.glkj.example.com`） |
| `GLKJ_SEARCH_BASE_URL` | 搜索端点（Anthropic 兼容基址，追加 `/messages`） |
| `GLH_HOME` | 覆盖用户主目录（默认 `~/.glh`） |
| `GLH_TELEMETRY_DISABLED` | 置 1 关闭会话遥测（遥测默认关闭且无内置上报地址） |

另有 `GLH_*` 受管命名空间（如 `GLH_SHELL`、`GLH_SESSION_ID`）由宿主注入 shell
与工具进程，不要手动设置。

## 5. 用户数据与配置目录

```
~/.glh/
├── .credentials.yaml   # Web UI 存储的密钥
├── .env                # 用户层环境变量
├── profiles/<name>/    # 每个 profile 的依赖与用户补丁（cordis.patch.yml）
└── skills/             # 用户级技能目录
```

项目级配置放在仓库的 `.glh/skills`、`.agents/` 与 `AGENTS.md` 等指令文件中。

## 6. Python SDK

```python
from deepseek_harness import DeepSeekHarnessClient, HarnessConfig

client = DeepSeekHarnessClient(HarnessConfig(
    base_url="https://你的网关地址",
    api_key="你的密钥",
))
```

SDK 通过环境变量（`GLKJ_API_KEY`、`GLKJ_BASE_URL`、`GLH_CORDIS_CONFIG` 等）驱动
内置 Node 运行时。本地开发需先构建运行时：

```sh
npx tsx scripts/build-exe-for-python-sdk.ts --targets=node22-linux-x64
cd python/sdk && uv run --with pytest pytest -q
```

## 7. 开发与测试常用脚本

| 脚本 | 作用 |
| --- | --- |
| `pnpm glh <cmd>` | 从源码运行 CLI |
| `pnpm test` | 全量单元/集成测试（vitest） |
| `pnpm run test:e2e` | 真实 API e2e（无 `GLKJ_API_KEY` 自动跳过） |
| `pnpm run test:snapshot` | Web UI 快照回放测试 |
| `pnpm run build` | 构建库产物与 Web 前端 |
| `pnpm run doc-sync` | 文档门禁（翻译配对、目录一致性等 28 项） |
| `pnpm run verify-translation-pairing` | 中英文档配对校验 |

## 8. 常见问题

- **启动报 `--profile <name> is required`**：直接运行 `glh` 不带 profile 时需要
  显式指定，或使用 `glh web` 快捷命令。
- **模型请求 404/连不上**：检查 `GLKJ_BASE_URL` 是否仍为占位地址；改为公司网关
  或 DeepSeek 官方地址。
- **迁移自旧版本**：配置目录由 `~/.dsh` 改为 `~/.glh`，环境变量前缀由 `DSH_*`/
  `DEEPSEEK_*` 改为 `GLH_*`/`GLKJ_*`，迁移时需同步更新脚本与 CI secrets。
