# GLKJ Harness

English | [中文](README.zh.md)

GLKJ Harness (`glh`) is an agent harness customized and maintained by GLKJ. It is based on [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (MIT).

It uses an architecture where **everything is a plugin**, and is powered by [Cordis](https://github.com/cordiverse/cordis), whose design is described in [_A Programming Paradigm for Spatiotemporal Composability_](https://github.com/cordiverse/paper).

## Developer preview

GLKJ Harness is currently in _developer preview_ and is iterating rapidly. **THERE WILL BE COMPATIBILITY-BREAKING CHANGES.**

## Run

### Run from `npm`

Install `Node.js`, then run:

```sh
npx @deepseek-ai/dsh web
```

The command starts the Web UI, served at `http://127.0.0.1:3080` by default. See [Web UI guide](docs/user/guide/index.md).

### Run from source

To run from a repository checkout:

```sh
git clone <GLKJ-repo-url>
cd deepseek-harness
pnpm install
pnpm run build
pnpm glh web
```

## Community and support

<!-- TODO(GLKJ): replace with the company's own support channels. -->

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Development

Start with the [development guide](docs/development.md) and [architecture documentation](docs/architecture.md).

For agents, follow [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE) — based on DeepSeek Harness by DeepSeek AI.

Third-party dependencies and their licenses are disclosed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
