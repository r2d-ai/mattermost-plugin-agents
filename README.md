<div align="center">

# Mattermost Agents Plugin [![Download Latest Release](https://img.shields.io/badge/Download-Latest%20Release-blue)](https://github.com/r2d-ai/mattermost-plugin-agents/releases/latest)

The Mattermost Agents Plugin integrates AI capabilities directly into your [Mattermost](https://github.com/mattermost/mattermost) workspace. **Run any local LLM** on your infrastructure or connect to cloud providers - you control your data and deployment.

</div>

![The Mattermost Agents AI Plugin is an extension for mattermost that provides functionality for self-hosted and vendor-hosted LLMs](img/mattermost-ai-llm-access.webp)

## Key Features

- **Multiple AI Assistants**: Configure different agents with specialized personalities and capabilities
- **Thread & Channel Summarization**: Get concise summaries of long discussions with a single click
- **Action Item Extraction**: Automatically identify and extract action items from threads
- **Meeting Transcription**: Transcribe and summarize meeting recordings
- **Semantic Search**: Find relevant content across your Mattermost instance using natural language
- **Smart Reactions**: Let AI suggest contextually appropriate emoji reactions
- **Direct Conversations**: Chat directly with AI assistants in dedicated channels
- **Flexible LLM Support**: Use local models (Ollama, vLLM, etc.), cloud providers (OpenAI, Anthropic, Azure), or any OpenAI-compatible API

## Documentation

Comprehensive documentation is available in the `/docs` directory:

- [User Guide](docs/user_guide.md): Learn how to interact with AI features
- [Admin Guide](docs/admin_guide.md): Detailed installation and configuration instructions
- [Load Testing](docs/load-testing.md): Configure the in-process mock LLM and load-test-ng Agents controller
- [Provider Setup](docs/providers.md): Configuration for supported LLM providers
- [Feature Documentation](docs/features/): Detailed guides for individual features
  - [Channel Summaries](docs/features/channel_summaries.md): Use the **Ask Agents about this channel** button to summarize and analyze channel activity

## Installation

1. Download the plugin `.tar.gz` asset from this fork's [releases page](https://github.com/r2d-ai/mattermost-plugin-agents/releases).
2. Upload and enable the plugin through the Mattermost System Console
3. Configure your desired LLM provider settings

### System Requirements

- Mattermost Server v12.0.0+
- PostgreSQL database with pgvector extension for semantic search capabilities
- Network access to your chosen LLM provider

## Quick Start

After installation, complete these steps to get started:

1. Navigate to **System Console > Plugins > Agents**
2. Create an agent and configure it with your LLM provider credentials
3. Set permissions for who can access the agent
4. Open the Agents panel from any channel using the AI icon in the right sidebar
5. Start interacting with your AI assistant

For detailed configuration instructions, see the [Admin Guide](docs/admin_guide.md).

## Integration

### Bridge Client

The plugin provides a Go client library for other Mattermost plugins and the Mattermost server to interact with the AI plugin's LLM Bridge API. This allows you to easily add AI capabilities to your own plugins or server-side features.

See the [Bridge Client README](public/bridgeclient/README.md) for installation and usage instructions.

## Development

### Prerequisites

- Go 1.26.7+ (see `go.mod`)
- Node.js 24.11+
- Access to an LLM provider (OpenAI, Anthropic, etc.)

### Local Setup

1. Setup your Mattermost development environment by following the [Mattermost developer setup guide](https://developers.mattermost.com/contribute/server/developer-setup/). If you have a remote mattermost server you want to develop to you can skip this step. 

2. Setup your Mattermost plugin development environment by following the [Plugin Developer setup guide](https://developers.mattermost.com/integrate/plugins/developer-setup/).

3. Clone the repository:
```bash
git clone https://github.com/r2d-ai/mattermost-plugin-agents.git
cd mattermost-plugin-agents
```

4. **Optional**. If you are developing to a remote server, setup environment variables to deploy:
```bash
MM_SERVICESETTINGS_SITEURL=http://localhost:8065
MM_ADMIN_USERNAME=<YOUR_USERNAME>
MM_ADMIN_PASSWORD=<YOUR_PASSWORD>
```

5. Run deploy to build the plugin
```bash
make deploy
```

### Other make commands

- Run `make help` for a list of all make commands
- Run `make check-style` to verify code style
- Run `make test` to run the test suite
- Run `make e2e` to run the e2e tests
- Run `make evals` to run prompt evaluations interactively (with TUI)
- Run `make evals-ci` to run prompt evaluations in CI mode (non-interactive)

### Release builds

The [Release workflow](.github/workflows/release.yml) automatically builds and uploads installable assets whenever a GitHub release or prerelease is published in this fork.

1. Merge the changes to release, including the workflow, into `master`.
2. Create a GitHub release with a version tag such as `v2.10.1-r2d.1` targeting that commit, then publish it. Tags must use `vMAJOR.MINOR.PATCH`, optionally followed by prerelease or build metadata.
3. Wait for the **Release** workflow to finish. The release's **Assets** will contain `mattermost-ai-<version>.tar.gz` and `SHA256SUMS`.

The single plugin archive includes Linux amd64/arm64, macOS amd64/arm64, and Windows amd64 server binaries plus the production webapp. Its manifest version comes from the selected tag, including when multiple tags point to the same commit. The build uses Go from `go.mod` and Node from `.nvmrc`, and needs no custom repository secrets. Only the asset upload job receives `contents: write` through GitHub's automatic token.

To rebuild an existing release, open **Actions → Release → Run workflow** and enter its exact tag. A rerun replaces only the generated archive and `SHA256SUMS` with the same names. Releases created by another workflow using `GITHUB_TOKEN` need this manual run because [GitHub suppresses recursive workflow triggers](https://docs.github.com/en/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/triggering-a-workflow). If release immutability is enabled, run the workflow for an existing draft before publishing; a published immutable release cannot accept new assets.

Download both assets into the same directory to verify the archive:

```bash
sha256sum --check SHA256SUMS
```

### Benchmark Tests

The streaming code has benchmark tests to measure performance and detect regressions:

```bash
# Run all streaming benchmarks
go test -bench=. -benchmem ./llm/... ./streaming/...

# Run specific benchmark
go test -bench=BenchmarkStreamToPost -benchmem ./streaming/...

# Run with CPU profiling
go test -bench=BenchmarkReadAll -cpuprofile=cpu.prof ./llm/...
```

Benchmarks cover:
- `ReadAll()` stream consumption with varying response sizes
- `TokenUsageLoggingWrapper` interception overhead
- `StreamToPost()` full processing pipeline with WebSocket events

### Multi-Provider Evaluation Support

The evaluation system supports testing with multiple LLM providers: OpenAI, Anthropic, and Azure OpenAI. By default, evaluations run against all providers, but you can target specific ones:

```bash
# Run with all providers (default)
make evals

# Run with only Anthropic
LLM_PROVIDER=anthropic make evals

# Run with OpenAI and Azure
LLM_PROVIDER=openai,azure make evals

# Use a specific model
ANTHROPIC_MODEL=claude-3-opus-20240229 make evals
```

See `cmd/evalviewer/README.md` for complete documentation on environment variables and configuration options.


## License

This repository is licensed under [Apache-2](./LICENSE), except for the [server/enterprise](server/enterprise) directory which is licensed under the [Mattermost Source Available License](LICENSE.enterprise). See [Mattermost Source Available License](https://docs.mattermost.com/overview/faq.html#mattermost-source-available-license) to learn more.
