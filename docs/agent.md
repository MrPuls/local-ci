# Claude Code Agent

Local CI ships as a [Claude Code](https://claude.ai/code) plugin, providing an agent that can autonomously run CI/CD pipelines on your behalf. This gives Claude Code the ability to validate code in real target environments — different runtimes, OS images, or with real services like databases.

## Installation

### Prerequisites

- [local-ci](https://github.com/MrPuls/local-ci) installed on your machine
- Docker installed and running

### Install the Plugin

```bash
# Add the marketplace
/plugin marketplace add MrPuls/local-ci

# Install the plugin
/plugin install local-ci@MrPuls-local-ci
```

### Update the Plugin

```bash
/plugin marketplace update MrPuls-local-ci
```

## What the Agent Can Do

- Write `.local-ci.yaml` configs tailored to the project's stack
- Run pipelines and interpret the results
- Validate that code builds in specific environments (e.g. different Go/Node/Python versions)
- Spin up infrastructure via bootstrap for integration testing
- Tear down per-job infrastructure via job-level bootstrap/cleanup
- Debug failing jobs by analyzing container output
- Run individual jobs for fast iteration

## How It Works

When Claude Code determines that environment-level validation is needed, it spawns the `local-ci` agent in the background. The agent:

1. Inspects the project to understand the build system and dependencies
2. Writes or reuses a `.local-ci.yaml` config
3. Runs the pipeline via `local-ci run`
4. Streams and analyzes the output in real time
5. Reports results back — which jobs passed, which failed, and why

Since the agent runs in the background, Claude Code can continue working with you while the pipeline executes. Docker image pulls are cached locally, so subsequent runs are fast.

## Agent Configuration

The agent is defined in `agents/local-ci.md` with the following settings:

| Setting | Value | Description |
|---|---|---|
| `tools` | Read, Write, Bash, Glob, Grep, WebFetch | Tools the agent can use |
| `model` | inherit | Uses the same model as the parent conversation |
| `background` | true | Runs without blocking the main conversation |
| `maxTurns` | 30 | Maximum number of agent turns before stopping |

## Example Workflows

### Multi-version Build Validation

Ask Claude Code to verify your Go project builds on multiple Go versions:

```yaml
stages:
  - build

Build-Go-1.21:
  stage: build
  image: golang:1.21
  workdir: /app
  script:
    - go build ./...

Build-Go-1.22:
  stage: build
  image: golang:1.22
  workdir: /app
  script:
    - go build ./...
```

### Integration Testing with Services

Ask Claude Code to run tests against a real database:

```yaml
stages:
  - test

Test:
  stage: test
  image: golang:1.22
  workdir: /app
  network:
    host_access: true
  job_bootstrap:
    run:
      - docker compose -f docker-compose.test.yml up -d
    timeout: 3
  job_cleanup:
    run:
      - docker compose -f docker-compose.test.yml down
    timeout: 2
  script:
    - go test ./...
```

### Caching Dependencies

The agent uses caching to speed up repeated runs:

```yaml
stages:
  - build

Build:
  stage: build
  image: node:20-alpine
  workdir: /app
  cache:
    key: node-modules
    paths:
      - /app/node_modules
  script:
    - npm install
    - npm run build
```

## CLI Reference

The agent uses these commands internally:

```bash
# Run full pipeline
local-ci run

# Run with a specific config
local-ci run --config path/to/config.yaml

# Run a single job (used for fast iteration)
local-ci run --job JobName

# Run specific stage(s)
local-ci run --stage StageName

# Pass environment variables
local-ci run --env KEY=VALUE
```

## Troubleshooting

### Agent can't find local-ci

The agent will attempt to install `local-ci` via Homebrew or `go install` if it's not found. If both fail, install it manually:

```bash
# macOS
brew install --cask MrPuls/local-ci/local-ci

# Go install
go install github.com/MrPuls/local-ci/cmd/local-ci@latest
```

### Docker is not running

The agent requires Docker to be running. Start Docker Desktop or the Docker daemon before invoking the agent.

### Agent runs out of turns

The agent has a 30-turn limit. For complex pipelines with many failures to debug, you may need to invoke it multiple times. Use `--job` to target specific failing jobs rather than re-running the entire pipeline.

## Further Reading

- [YAML Configuration Reference](yaml-reference.md) — Full config syntax and options
- [Technical Reference](tech-reference.md) — Architecture, execution flow, and advanced features
