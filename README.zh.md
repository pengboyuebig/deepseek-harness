# GLKJ Harness

[English](README.md) | 中文

GLKJ Harness（`glh`）是由 GLKJ 定制并维护的 agent harness（智能体框架），基于 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（MIT 许可证）。

它采用**一切皆插件**的架构，并由 [Cordis](https://github.com/cordiverse/cordis) 驱动，其设计参见论文 [_A Programming Paradigm for Spatiotemporal Composability_](https://github.com/cordiverse/paper)。

## 开发者预览

GLKJ Harness 目前处于 _开发者预览_ 阶段，正在快速迭代。**未来将出现破坏兼容性的变更。**

## 运行

### 通过 `npm` 运行

安装 `Node.js`，然后运行：

```sh
npx @deepseek-ai/dsh web
```

该命令会启动 Web UI，默认地址为 `http://127.0.0.1:3080`。详见 [Web UI 指南](docs/user/guide/index.md)。

### 从源码运行

如需从仓库源码运行：

```sh
git clone <GLKJ 仓库地址>
cd deepseek-harness
pnpm install
pnpm run build
pnpm glh web
```

## 社区与支持

<!-- TODO(GLKJ)：替换为公司自己的支持渠道。 -->

## 参与贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开发

请先阅读[开发指南](docs/development.md)与[架构文档](docs/architecture.md)。

面向 agent：请遵循 [AGENTS.md](AGENTS.md)。

## 许可证

[MIT](LICENSE) — 基于 DeepSeek AI 的 DeepSeek Harness。

第三方依赖及其许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
