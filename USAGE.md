# GLKJ Harness 使用方法

GLKJ Harness（命令 `glh`）是基于 DeepSeek Harness 定制的插件化智能体框架。本文件面向使用者与本地开发者；生产部署、完整 npm 包族发布和 Python 运行时构建见 [BUILD.md](BUILD.md)。品牌占位素材替换见 [BRANDING.md](BRANDING.md)。

## 1. 使用前须知

- Node.js：`^22.19.0 || >=24.0.0`。
- pnpm：源码安装、构建和开发模式使用仓库固定的 `pnpm@11.7.0`。
- GLKJ Web 默认仅监听 `http://127.0.0.1:3080`；当前不支持直接以 `--host 0.0.0.0` 对外暴露。远程访问请使用受认证的反向代理或 SSH 隧道，并让 GLKJ 保持 loopback 监听。
- GLKJ **不内置可用的默认模型**。首次进入 Web UI 后，必须配置供应商并显式选择模型；未选择时编辑器会阻止发送，不会访问占位 API。
- 不要使用 `npx @deepseek-ai/dsh web`。公共 npm 上同名包属于上游 DeepSeek 产品，不是本 GLKJ 分支；当前可靠的交付方式是源码构建部署，或发布完整包族到公司的私有 Registry。

## 2. 快速启动：从源码运行

适用于本地使用、测试和开发。全新 checkout 必须先构建一次；`pnpm glh` 不会自动生成 Host、Client 或 Web 前端产物。

```sh
git clone <GLKJ-repo-url>
cd deepseek-harness
pnpm install --frozen-lockfile
pnpm run build
pnpm glh web
```

当控制台显示以下地址后再访问浏览器：

```text
glh web: http://127.0.0.1:3080
```

打开 <http://127.0.0.1:3080/>。按 `Ctrl+C` 可正常停止服务；监督进程可发送 `SIGTERM`，服务会优雅释放资源后退出。

> `pnpm glh web` 从 TypeScript 源码入口启动，适合源码 checkout。生产进程请使用构建后的 `node apps/cli/lib/bin.js web`，见 [BUILD.md](BUILD.md)。

## 3. 开发环境：Web 前端热更新

Web 开发需要两个终端，并且必须从**同一个仓库 checkout**运行。首次使用或修改 Host、普通包、Web shell 后，先完整构建：

```sh
pnpm run build
```

然后分别启动：

```sh
# 终端 1：启动源码 Host 与 GLKJ Web 服务
pnpm glh web
```

```sh
# 终端 2：监听并重建 web client-plugin bundle
pnpm run dev:web
```

`pnpm run dev:web` 已使用轮询模式，适用于文件事件不可靠的挂载目录。它只监听声明为 Web `dsh.client` 的客户端插件，并将对应 `lib/client.js` 重新构建；GLKJ Web 服务已内置 HMR 接收器，不需要也不能用独立 Vite 服务替代。

以下变更通常需要重新执行 `pnpm run build` 并刷新页面，必要时重启 `pnpm glh web`：

- `apps/web` 的页面 shell、Vite 配置或静态资源；
- Host 代码、组合配置或普通 Node 包；
- 新增一个 Web client-plugin（watcher 只在启动时发现包集合）。

停止开发环境时，在两个终端分别按 `Ctrl+C` 即可。

## 4. 模型接入与首次选择

默认地址 `https://api.glkj.example.com` 只是占位示例，不能作为正式模型服务。配置 API Key 或 Base URL 本身**不会**自动选中模型。

### 方式一：使用环境变量

在启动进程的 shell 中设置真实网关：

```sh
export GLKJ_API_KEY="你的密钥"
export GLKJ_BASE_URL="https://你的公司网关/v1"
pnpm glh web
```

启动后仍需在 Web UI 的 **Settings → Models** 中选择一个已配置模型。环境变量适合部署注入、临时测试或受控服务账户。

### 方式二：通过 Web UI 配置

1. 打开 **Settings → Models**。
2. 添加或编辑供应商；自定义供应商需要填写匹配协议的 Base URL、API Key 和至少一个模型。
3. 保存配置。
4. 在会话顶部或输入框附近的模型选择器中，选中该模型。
5. 再发送消息。

Web UI 保存的密钥位于 `$GLH_HOME/.credentials.yaml`，模型和供应商设置位于 `$GLH_HOME/settings.yaml`。保存后的设置会热更新；轮换密钥或修正 Base URL 通常不需要重启服务。

### 常用环境变量

| 变量 | 说明 |
| --- | --- |
| `GLKJ_API_KEY` | 默认模型服务的 API 密钥 |
| `GLKJ_BASE_URL` | 默认模型 API 基址；未设置时为占位地址 |
| `GLKJ_SEARCH_BASE_URL` | 搜索服务的 Anthropic 兼容基址 |
| `GLH_HOME` | 用户状态目录，默认 `~/.glh` |
| `GLH_TELEMETRY_DISABLED` | 设置为非空值时强制关闭遥测 |

`GLH_*` 中还有由宿主注入工具进程的受管变量，例如 `GLH_SHELL`、`GLH_SESSION_ID`；不要手动设置它们。

## 5. 常用 CLI 命令

| 命令 | 作用 |
| --- | --- |
| `pnpm glh web` | 从源码启动 Web UI，默认 `127.0.0.1:3080` |
| `pnpm glh web --port 8080` | 从源码以指定端口启动 Web UI |
| `pnpm glh --profile tui` | 启动终端 UI |
| `pnpm glh --profile headless "任务描述"` | 无界面执行单个任务并输出最终答复 |
| `pnpm glh web --dump-config` | 打印有效组合配置，不启动服务 |
| `pnpm glh plugin --profile tui add <pkg>` | 向指定 profile 安装插件 |
| `pnpm glh -V` | 查看版本 |

运行安装后的完整私有 Registry 包族时，将上述 `pnpm glh` 改为 `glh`；运行源码构建后的生产 JS 时，将其改为 `node apps/cli/lib/bin.js`。两种方式的选择见 [BUILD.md](BUILD.md)。

## 6. 用户数据、配置与备份

默认状态目录为 `~/.glh/`，可通过 `GLH_HOME` 改到服务账户专用目录：

```text
~/.glh/
├── .credentials.yaml   # Web UI 保存的密钥；应限制文件权限
├── .env                # 用户层环境变量
├── settings.yaml       # 模型与供应商设置
├── profiles/<name>/    # profile 依赖及 cordis.patch.yml 覆盖层
└── skills/             # 用户级技能目录
```

生产服务建议显式设置 `GLH_HOME`，例如 `/var/lib/glkj-harness`，并仅授予服务账户访问权限。备份时按公司的密钥管理策略处理 `.credentials.yaml`；不要将它提交到 Git、写入镜像或输出到日志。

## 7. 常见问题

| 现象 | 处理 |
| --- | --- |
| 编辑器提示尚未选择模型 | 在 **Settings → Models** 配置供应商后，在会话模型选择器中显式选中模型。 |
| 本轮运行失败：`404 status code (no body)` | 这是选择成功后实际调用的上游模型服务返回 404；检查自定义供应商的协议、Base URL 和路径是否匹配。常见错误是重复或遗漏 `/v1`，或把 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 协议配错。 |
| 启动时提示前端未构建 | 在仓库根目录执行 `pnpm run build`，再启动。 |
| 访问 3080 出现本地空响应 404 | 确认 `pnpm glh web` 已打印就绪 URL；使用完整页面刷新，并检查实际访问的是 `http://127.0.0.1:3080/` 而非其他开发服务器端口。 |
| 端口已被占用 | 使用 `pnpm glh web --port 8080`，或停止占用 3080 的进程。 |
| 从 npm 安装后出现上游导出不匹配 | 不要安装单独的 CLI tarball 或公共 npm 同名包；使用源码构建部署，或让管理员发布完整包族到私有 Registry。 |

## 8. 测试与质量命令

| 命令 | 作用 |
| --- | --- |
| `pnpm run build` | 构建 Host/Client 库与 Web 前端 |
| `pnpm test` | 运行全量 Vitest 单元与集成测试 |
| `pnpm run test:e2e` | 运行真实 API e2e；未配置 `GLKJ_API_KEY` 时相关测试会跳过 |
| `pnpm run test:snapshot` | 运行 Web UI 快照回放测试 |
| `pnpm run test:web` | 构建后运行浏览器 e2e；需要 Playwright Chromium 与系统依赖 |
| `pnpm run doc-sync` | 运行文档一致性门禁 |
| `pnpm run verify-translation-pairing` | 校验中英文档配对记录 |

Python SDK、单文件运行时和 wheel 的使用说明见 `python/sdk/README.zh.md`；构建与发布说明见 [BUILD.md](BUILD.md)。
