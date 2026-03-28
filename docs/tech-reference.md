# Technical Reference and CLI Usage

## Table of Contents
- [Command Line Interface](#command-line-interface)
- [Architecture](#architecture)
- [Pipeline Execution Flow](#pipeline-execution-flow)
   - [1. Configuration Loading and Validation](#1-configuration-loading-and-validation)
   - [2. Pipeline Orchestration](#2-pipeline-orchestration)
   - [3. Job Execution](#3-job-execution)
- [Technical Details](#technical-details)
   - [Container Management](#container-management)
   - [Environment Variables](#environment-variables)
   - [Remote Provider](#remote-provider)
   - [File System Handling](#file-system-handling)
   - [Caching System](#caching-system)
   - [Job-Specific Execution](#job-specific-execution)
   - [Stage-Specific Execution](#stage-specific-execution)
   - [Graceful Shutdown](#graceful-shutdown)
   - [Remote Clone and Execution](#remote-clone-and-execution)
   - [Bootstrap](#bootstrap)
   - [Cleanup](#cleanup)
- [Claude Code Agent](#claude-code-agent)
- [Limitations and Notes](#limitations-and-notes)

## Command Line Interface

Local CI features a simple command-line interface:

```bash
# Run pipeline with default config
local-ci run

# Run pipeline with custom config file
local-ci run --config my-config.yaml

# Run specific job only
local-ci run --job JobName

# Show version information
local-ci version

# Get help
local-ci --help
```

## Architecture

Local CI is built using a clean, modular architecture:

1. **CLI Layer** - Handles command parsing and user interaction
2. **Application Layer** - Orchestrates pipeline execution
3. **Pipeline Layer** - Manages stages and job execution
4. **Execution Layer** - Handles container operations
5. **Configuration Layer** - Parses and validates YAML

## Pipeline Execution Flow

The tool operates in several stages:

### 1. Configuration Loading and Validation
- Reads and parses the YAML configuration
- Validates required fields and relationships
- Resolves global and job-specific variables
- Creates job models for execution

### 2. Pipeline Orchestration
- Organizes jobs by stages
- Executes stages in sequence (as defined in configuration)
- Manages execution flow and error handling

### 3. Job Execution
For each job:

1. **Container Setup**:
   - Pulls the specified Docker image
   - Creates a container with:
      - Custom working directory (defaults to "/")
      - Merged environment variables (global + job-specific)
      - Cache volume mounts if specified
      - Script commands joined for sequential execution

2. **File System Handling**:
   - Creates a tar archive of the project files (respecting .gitignore)
   - Copies project files into the container

3. **Execution**:
   - Starts the container
   - Streams logs in real-time to stdout
   - Waits for container completion

4. **Cleanup**:
   - Removes the container after execution
   - Preserves cache volumes for reuse in future runs

## Technical Details

### Container Management
- Uses Docker SDK for Go
- One-hour timeout per container
- Context-aware operations with proper resource cleanup
- Real-time log streaming

### Environment Variables
Environment variables from global and job-specific contexts are merged:

1. Global variables are applied first
2. Job-specific variables override globals with the same name
3. Variables which start with `$` will be treated as local variables, so for example if you need to pass a variable from your local environment, you can prefix it with `$` and it will become available in the job environment.

Example:
```yaml
variables:
  GLOBAL_VAR: "global_value"

JobName:
  variables:
    JOB_VAR: "job_value"
    GLOBAL_VAR: "override_value"  # Overrides the global
```

The job environment would contain:
```
GLOBAL_VAR=override_value
JOB_VAR=job_value
```

### Remote Provider
Provide your GitLab host name and access token for local-ci to access and inject your project's environment variables.

Received this way, variables are treated as global variables af is specified through high level ```variables``` keyword.

Provided hostname allows for modification of docker's auth credential, hence providing access to custom/self-hosted registries. In case of GitLab, allows for accessing images saved in private registries without needing to configure Docker credentials separately.

### File System Handling
The tool provides smart file system handling with .gitignore support:

1. **File Collection**:
   - Reads `.gitignore` if present in the project root
   - Respects all non-comment patterns
   - Creates a tar archive containing only relevant files

2. **Container Integration**:
   - Copies the archive to the container's working directory
   - Preserves file metadata and permissions

### Caching System
Local CI provides a caching system for faster builds:

1. **Cache Configuration**:
   - Specify a unique cache key
   - Define paths to cache

2. **Implementation**:
   - Creates Docker volumes for persistent storage
   - Mounts volumes to the specified paths
   - Preserves cached data between job runs

Example:
```yaml
JobName:
  cache:
    key: my-cache-key
    paths:
      - node_modules/
      - .npm/
```

### Job-Specific Execution

Local CI allows running individual jobs instead of the full pipeline:

```bash
local-ci run --job JobName
```
Or
```bash
local-ci run --job JobName1,JobName2
```
To run several jobs

#### How It Works

When running a specific job:

1. **Bypasses Stage Ordering**:
   - Only the specified job runs, regardless of its stage
   - Stage dependencies are not enforced

2. **Direct Execution**:
   - The job is extracted directly from the configuration
   - All job features (caching, environment variables, etc.) work normally

3. **Standard Error Handling**:
   - Error handling and cleanup remain consistent with pipeline execution

### Stage-Specific Execution

Local CI allows running individual jobs instead of the full pipeline:

```bash
local-ci run --stage StageName
```
Or
```bash
local-ci run --stage StageName1,StageName2
```
To run jobs marked with specified stages

#### How It Works

When running jobs by a specific stage:

1. **Direct Execution**:
    - The jobs are extracted directly from the configuration
    - All jobs features (caching, environment variables, etc.) work normally

2. **Standard Error Handling**:
    - Error handling and cleanup remain consistent with pipeline execution

#### Use Cases

Running specific jobs is useful for:

- Debugging problematic jobs without running the entire pipeline
- Testing configuration changes quickly
- Running utility or deployment jobs independently
- Fast iteration during development

#### Example

Given this configuration:

```yaml
stages:
  - build
  - test
  - deploy

Build:
  stage: build
  image: golang:1.21
  script:
    - go build -o myapp

Test:
  stage: test
  image: golang:1.21
  script:
    - go test ./...

Deploy:
  stage: deploy
  image: alpine
  script:
    - echo "Deploying..."
```

You can run just the Test job:

```bash
local-ci run --job Test
```

This will execute only the Test job, skipping the Build and Deploy stages.

### Graceful Shutdown

Local CI implements graceful shutdown handling that ensures resources are properly cleaned up when execution is interrupted. This is particularly useful when:

- User presses Ctrl+C to stop execution
- The terminal process is killed
- The system is shutting down

#### Implementation Details

The graceful shutdown system:

1. **Catches Interruption Signals**:
    - Listens for SIGINT (Ctrl+C) and SIGTERM signals
    - Provides a user-friendly message about the shutdown process

2. **Cancels Running Operations**:
    - Uses context cancellation to signal all operations to stop
    - Propagates cancellation to Docker operations
    - Ensures in-progress tasks can exit cleanly

3. **Resource Cleanup**:
    - Stops all running containers
    - Removes containers to prevent resource leaks
    - Closes connections and releases resources
    - Sets a timeout to prevent hanging during cleanup

#### How It Works

When you press Ctrl+C during execution:

1. The signal is caught by the orchestration layer
2. A message is displayed indicating that graceful shutdown has begun
3. Running operations are cancelled via context
4. The cleanup process stops and removes all containers
5. The program exits with a clean state

Example output:
```
Starting job: Build
Pulling image: golang:1.21
^C
Operation interrupted, initiating graceful shutdown...
Stopping runner...
Starting cleanup...
Containers found: 1
Deleting container: "abc123def456", ["/local-ci-build"]
All containers removed!
```

#### Technical Architecture

The graceful shutdown feature is implemented through:

1. **Orchestration Layer**:
    - Manages signal handling and high-level workflow
    - Coordinates between execution and cleanup
    - Ensures cleanup has sufficient time to complete

2. **Runner Cleanup**:
    - Identifies all containers created by Local CI
    - Executes Docker stop and remove operations
    - Logs cleanup progress for visibility

3. **Context Propagation**:
    - All operations receive a cancellable context
    - Operations check for cancellation and exit cleanly
    - Prevents resource leaks and hanging processes

This architecture ensures that Local CI behaves well even when interrupted, leaving your system in a clean state without orphaned containers or resources.

## Remote Clone and Execution

Local CI supports cloning the repository and executing the pipeline configuration from a remote URL.

```bash
local-ci run --remote <repository_url>
```

This will `git clone` the repositiry to `~/.local/shared/local-ci/<repository_name>` and run the yaml configuration from that location. If the repository is already cloned, Local CI will `git pull` to update the existing clone. Currently, only the main branch is supported but branch switching is planned.

## Bootstrap

Bootstrap runs host-level setup commands before any job containers are started. Each command is executed sequentially on the host machine with a shared timeout context — if the timeout is exceeded, remaining commands are cancelled and the pipeline does not proceed.

Timeout is specified as an integer representing minutes. If not provided, defaults to 5 minutes.

Example output:
```
Running bootstrap with timeout 5 minutes
Running bootstrap command: docker compose -f docker-compose.yml up -d
```

## Cleanup

Cleanup is the counterpart to bootstrap — it runs host-level teardown commands after the pipeline finishes. Cleanup runs regardless of whether the pipeline succeeded or failed, ensuring that infrastructure started during bootstrap is always torn down.

### Key Differences from Bootstrap

| | Bootstrap | Cleanup |
|---|---|---|
| **When** | Before any jobs | After all jobs (or on failure/interruption) |
| **Error handling** | Fails fast — pipeline stops if a command fails | Best-effort — logs errors but continues through all commands |
| **Required** | No | No (but requires bootstrap to be defined) |

### How It Works

1. Cleanup is registered via `defer` immediately after bootstrap succeeds
2. This guarantees it runs on every exit path: success, job failure, or signal interruption (Ctrl+C)
3. Each command runs sequentially with a shared timeout context
4. If a command fails, the error is logged and the remaining commands still execute

Timeout is specified as an integer representing minutes. If not provided, defaults to 5 minutes.

### Example Configuration

```yaml
bootstrap:
  run:
    - docker compose -f docker-compose.yml up -d
  timeout: 5

cleanup:
  run:
    - docker compose -f docker-compose.yml down
    - docker volume prune -f
  timeout: 5
```

Example output:
```
Running cleanup with timeout 5 minutes
Running cleanup command: docker compose -f docker-compose.yml down
Running cleanup command: docker volume prune -f
```

## Claude Code Agent

Local CI ships as a [Claude Code](https://claude.ai/code) plugin, providing an agent that can run pipelines on your behalf. This gives Claude Code the ability to validate code in real target environments — different runtimes, OS images, or with real services like databases.

### Installation

Requires `local-ci` to be installed on your machine ([see above](#command-line-interface)) and Docker to be running.

Install the plugin
```bash
/plugin install local-ci@MrPuls-local-ci
```

To update the plugin after a new release:
```bash
/plugin marketplace update MrPuls-local-ci
```

### What the Agent Can Do

- Write `.local-ci.yaml` configs tailored to the project's stack
- Run pipelines and interpret the results
- Validate that code builds in specific environments (e.g. different Go/Node/Python versions)
- Spin up infrastructure via bootstrap for integration testing
- Debug failing jobs by analyzing container output
- Run individual jobs for fast iteration

### How It Works

When Claude Code determines that environment-level validation is needed, it spawns the `local-ci` agent in the background. The agent:

1. Inspects the project to understand the build system and dependencies
2. Writes or reuses a `.local-ci.yaml` config
3. Runs the pipeline via `local-ci run`
4. Streams and analyzes the output in real time
5. Reports results back — which jobs passed, which failed, and why

Since the agent runs in the background, Claude Code can continue working with you while the pipeline executes. Docker image pulls are cached locally, so subsequent runs are fast.

### Example Workflow

You ask Claude Code to verify that your Go project builds on both Go 1.21 and Go 1.22. The agent creates a config with two jobs targeting different images and runs them:

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

The agent runs the pipeline, observes the results, and reports back whether both versions compiled successfully.

## Limitations and Notes

1. **Current Limitations**:

   - Single-node execution only
   - Sequential execution within stages
   - Fixed one-hour timeout
   - Job-specific execution bypasses stage dependencies
   - No Github integration similar to GitLab

2. **Future Enhancements**:

   - Parallel job execution within stages
   - Persistent services support
   - Git branch switching for cloned repositories via `--remote` command
   - Alias support for remote repository URLs
   - List command for available remote repositories and local clones
   - Installation via Homebrew
   - Remote execution support (run your pipeline on a remote machine)
