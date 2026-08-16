# GLKJ 品牌替换指南（BRANDING.md）

本仓库已从 DeepSeek Harness 品牌改造为 GLKJ 品牌（用户可见层）。
当前所有品牌素材均为**占位实现**；拿到公司正式素材后，按本清单逐项替换即可。

## 1. 待替换的占位项

### 1.1 Logo 与图形资产（5 处）

| 位置 | 内容 | 说明 |
| --- | --- | --- |
| `packages/client/ui-primitives/src/BrandMark.tsx` | 2×2 方格图标（Web UI 侧边栏、空状态页） | 用正式 logo 的 SVG path 重写，保持 `currentColor` 与组件签名 |
| `packages/client/ui-primitives/src/BrandWordmark.tsx` | 文字版 "GLKJ Harness"（系统字体 `<text>`） | 换成正式字标 path；建议保持 182×24 视窗避免布局位移 |
| `website/public/favicon.svg` | 方格图标 + `#4D6BFE` | 文档站 favicon |
| `apps/web/public/favicon.svg` | 方格图标（含深色模式 CSS） | Web 应用 favicon |
| `website/public/wordmark.svg` | 文字版 "GLKJ Harness" | 文档站导航字标（被内联进 `.vitepress/config.ts`） |

### 1.2 品牌主色（占位：沿用上游蓝）

当前 UI 主色仍为上游蓝 `#4D6BFE`（改动面太大，暂作占位）。
替换正式主色时全局搜索 `4D6BFE`（favicon、徽章、文档 shields 链接等处）。

### 1.3 产品显示名（占位："GLKJ Harness"）

正式名称确定后全局搜索 `GLKJ Harness` 替换，重点位置：

- `apps/web/index.html`（浏览器标签标题）
- `apps/web/public/manifest.webmanifest`（PWA 名称，简称现为 `GLKJ`）
- `website/docs.ts`、`website/.vitepress/config.ts`（文档站标题）
- `README.md` / `README.zh.md`
- `packages/client/ui-settings-models/src/onboarding-copy.ts`（首启双语弹窗文案）
- `packages/core/system-prompt/src/index.ts:361`（发给模型的系统提示词身份句）
- `packages/subagent/subagent-codex/src/wire.ts`（codex 子代理握手 clientInfo，现为 `glkj-harness`）

### 1.4 后端地址（占位：`https://api.glkj.example.com`）

拿到公司网关地址后全局搜索 `api.glkj.example.com`，共 3 类源头：

| 位置 | 用途 |
| --- | --- |
| `packages/llm/llm-deepseek/src/index.ts`（`PUBLIC_BASE_URL`） | LLM 默认端点（可用 `GLKJ_BASE_URL` 覆盖） |
| `packages/web/web-search-deepseek/src/provider.ts` 与 `src/index.ts` | 搜索端点（`/anthropic/v1` 后缀） |
| `packages/client/ui-settings-models/src/client/ProviderEditor.tsx:44` | 设置页占位地址 |

其他占位域名（一并替换）：
- `glkj.example.com` — User-Agent 中的产品主页（见 1.5）
- `git.glkj.example.com/glkj/harness` — README/docs 中的克隆地址，换成公司真实仓库
- `www.glkj.example.com` / `docs.glkj.example.com` — 演示数据与测试夹具

### 1.5 User-Agent（现为 `glkj-harness/0.1.0`）

- `packages/web/web-fetch-http/src/index.ts:25`（`DEFAULT_USER_AGENT`，含主页链接）
- `packages/web/web-search-exa/src/provider.ts:31`
- `packages/web/web-search-perplexity/src/provider.ts:34`
- `packages/web/web-search-deepseek/src/provider.ts:50`
- `.github/issue-management/policy.mjs`（现为 `glkj-issue-policy`）

### 1.6 环境变量前缀

- **`GLKJ_*`**：模型/搜索提供方配置（`GLKJ_API_KEY`、`GLKJ_BASE_URL`、`GLKJ_MODELS` 等）
- **`GLH_*`**：宿主受管命名空间（`GLH_HOME`、`GLH_SHELL`、`GLH_SESSION_ID`、构建期 `GLH_BUILD_FACE` 等），定义于 `packages/subprocess/subprocess/src/types.ts` 的 `GLH_ENV_PREFIX`

### 1.7 命令与目录

- CLI 命令：`glh`（`apps/cli/package.json` bin、根 `package.json` 脚本、`apps/cli/src/args.ts`）
- 配置主目录：`~/.glh`（`packages/util/home-paths/src/index.ts` 的 `GLH_HOME_DIR_NAME`）
- 项目级目录：`.glh/skills` 等（`packages/skill/skill-filesystem/src/index.ts`）

### 1.8 社区与仓库信息（待补）

- `README.md` / `README.zh.md` 的"社区与支持"小节是 TODO 占位，需填公司渠道
- CONTRIBUTING 中"无法接受外部 PR"等表述按公司策略调整

## 2. 有意保留的上游内容（不要替换）

- **npm 包名与 import**：全部 `@deepseek-ai/dsh-*`、`@deepseek-ai/cordis-*` 保留，
  以便后续合并上游更新（这是"用户可见层改造"的边界）。
- **插件清单 `dsh` 字段**：package.json 中的 `dsh` manifest section 是插件 API 兼容面。
- **wire 标记**：`window.__DSH_BOOT__`、`__DSH_MODULES__`、`__DSH_PERSISTENT_BASH_*__`
  是宿主与客户端的内部协议常量。
- **模型 ID**：`deepseek-v4` 等目录模型 ID 属配置数据，接入公司网关后在模型目录配置中调整。
- **Python SDK**：`deepseek-harness-sdk` / `deepseek_harness_runtime` 包名未改（二期可选）。
- **溯源**：LICENSE 保留上游 `Copyright (c) 2026 DeepSeek`（MIT 要求）；
  README 保留 "based on DeepSeek Harness" 说明。
- **vendor/**：上游 Cordis 包及 `docs/rescope.md` 映射关系。
- `.agents/notes/`：历史工程笔记按原样保留。

## 3. 遥测说明

会话遥测默认**关闭**且无内置上报地址；启用时由用户在配置中提供 OTLP 端点
（`packages/session/session-telemetry-otel`）。若公司需要默认遥测，在那里接入。

## 4. 历史提交

品牌改造在 `glkj/glkj_main` 分支上按阶段提交（brand(assets) → brand(cli) →
brand(backend) → brand(copy) → …），可用 `git log --oneline` 查看每步范围。
