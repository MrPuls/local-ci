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
   - [Per-Job Parallel Flag](#per-job-parallel-flag)
   - [Matrix Builds](#matrix-builds)
   - [Parallel Execution](#parallel-execution)
   - [Graceful Shutdown](#graceful-shutdown)
   - [Remote Clone and Execution](#remote-clone-and-execution)
   - [Bootstrap](#bootstrap)
   - [Cleanup](#cleanup)
   - [Job Bootstrap and Cleanup](#job-bootstrap-and-cleanup)
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

# Run all jobs in parallel
local-ci run --parallel

# Run stages in order, with jobs inside each stage in parallel
local-ci run --parallel-stages

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
- Reads and parses the main YAML configuration
- Recursively loads any `include:`d files (paths resolved relative to the including file; cycles rejected)
- Merges included configs into the main config with main-wins semantics
- Resolves `extends:` chains on every non-template job (left-to-right template merge, then local fields override)
- Drops `.dot-prefixed` templates from the final job list
- Validates required fields and relationships
- Resolves global and job-specific variables
- Creates job models for execution

See the [YAML Configuration Reference — Templates](yaml-reference.md#templates) for the user-facing semantics of `include:` and `extends:`.

### 2. Pipeline Orchestration
- Organizes jobs by stages
- Executes stages in sequence (as defined in configuration)
- Manages execution flow and error handling

### 3. Job Execution
For each job:

1. **Job Bootstrap** (if defined):
   - Runs host-level setup commands before the container starts
   - If a command fails, the pipeline stops

2. **Container Setup**:
   - Pulls the specified Docker image
   - Creates a container with:
      - Custom working directory (defaults to "/")
      - Merged environment variables (global + job-specific)
      - Cache volume mounts if specified
      - Script commands joined for sequential execution

3. **File System Handling**:
   - Creates a tar archive of the project files (respecting .gitignore)
   - Copies project files into the container

4. **Execution**:
   - Starts the container
   - Streams logs in real-time to stdout (in parallel mode, logs are written to per-job files instead — see [Parallel Execution](#parallel-execution))
   - Waits for container completion

5. **Cleanup**:
   - Removes the container after execution
   - Preserves cache volumes for reuse in future runs

6. **Job Cleanup** (if defined):
   - Runs host-level teardown commands after the job finishes
   - Runs regardless of whether the job succeeded or failed

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

### Per-Job Parallel Flag

In default sequential mode, individual jobs can opt out of the sequential chain by setting `parallel: true` in their config. Such jobs are launched at pipeline start and run in the background while the remaining jobs continue to execute one after another.

```yaml
stages:
  - build
  - test
  - deploy

Build:
  stage: build
  image: golang:1.22
  script:
    - go build ./...

Lint:
  stage: build
  image: golang:1.22
  parallel: true
  script:
    - go vet ./...

Test:
  stage: test
  image: golang:1.22
  script:
    - go test ./...
```

In this example, `Build` and `Test` run sequentially in stage order, while `Lint` starts alongside `Build` and finishes whenever it finishes — its result does not block `Test`.

#### How It Works

1. **Decoupled from stages**:
   - Jobs marked `parallel: true` ignore stage boundaries; they all start at pipeline launch regardless of which stage they belong to.
   - Remaining jobs (without the flag) run in their normal stage order.

2. **Output**:
   - Sequential jobs stream their output to stdout exactly like a normal run.
   - Detached jobs write their full output to `.local-ci/logs/<timestamp>/<job-name>.log`.
   - Each detached job prints one line when it starts (`[detached] Name: started → <path>`) and one line when it finishes (`[detached] Name: passed` or `[detached] Name: failed (see <path>)`).
   - There is no live status board in this mode — the sequential stream owns the terminal.

3. **Failure semantics**:
   - If a sequential job fails, the sequential chain stops, but the pipeline still waits for detached jobs to finish before exiting. This avoids leaving orphaned containers behind.
   - If a detached job fails, the sequential chain is unaffected. The pipeline exit code aggregates failures from both groups.

4. **Interaction with flags**:
   - `--parallel` and `--parallel-stages` ignore the keyword entirely — they have their own execution model.
   - `--job` and `--stage` filter first; the keyword only controls *how* a selected job runs, not whether it runs.

#### Notes

- Detached jobs that share a cache key with sequential or other detached jobs may corrupt the cache through concurrent writes. Avoid sharing cache keys between jobs that can run at the same time.
- The `.local-ci/` directory should be added to `.gitignore` so run logs are not committed.

### Matrix Builds

A job can declare a `matrix:` block to fan out into multiple variants, each running with a different combination of variable values. Each variant is its own independent job with the matrix values merged into its `variables` map.

```yaml
stages:
  - test

Test:
  stage: test
  image: golang:1.22
  matrix:
    - GO_VERSION: ["1.21", "1.22"]
      OS: [linux, alpine]
  script:
    - go test ./...
```

The example above expands `Test` into four variants:
- `Test_GO_VERSION.1.21_OS.alpine`
- `Test_GO_VERSION.1.21_OS.linux`
- `Test_GO_VERSION.1.22_OS.alpine`
- `Test_GO_VERSION.1.22_OS.linux`

Each runs with `GO_VERSION` and `OS` set in its environment.

#### Syntax

`matrix:` is a list of entries. Each entry is a map of variable name to value(s). Within an entry:
- A scalar value (`PROVIDER: aws`) becomes a single fixed value.
- A list value (`REGION: [us-east, eu-west]`) is expanded as part of the cartesian product.

Multiple entries let you build asymmetric matrices (different variable sets per entry):

```yaml
Deploy:
  matrix:
    - PROVIDER: aws
      REGION: [us-east, us-west]
    - PROVIDER: ovh
      REGION: [eu-west]
```

This produces three variants — `aws/us-east`, `aws/us-west`, and `ovh/eu-west` — without generating the `ovh/us-east` combination.

#### Variant naming

Variants are named `<JobName>_<key>.<value>_<key>.<value>...` with keys sorted alphabetically for determinism. Matrix keys and values must match `[a-zA-Z0-9_.-]+` — invalid characters are rejected at config load. This restriction exists because variant names are used directly as Docker container names.

#### Execution

How variants run depends on the active mode:

| Mode | Behavior |
|---|---|
| Default sequential | Variants form a **parallel barrier**: they run concurrently with a live status board, and the sequential chain waits for the whole group to finish before continuing. |
| `parallel: true` on parent | Variants inherit the flag and run as **detached** background jobs individually (no barrier). |
| `--parallel` | Variants join the global parallel pool — no special treatment. |
| `--parallel-stages` | Variants stay in their parent's stage group. |

#### Failure semantics

Within a barrier, all variants run to completion even if one fails. The aggregate error stops the sequential chain after the barrier finishes. Detached variants follow the standard detached behavior (their failure does not stop sequential jobs).

#### Filtering

`--job Test` selects the base job before expansion, so all of `Test`'s variants run. To run just one variant, you would need to write a config without that matrix entry — there is no per-variant CLI selector in this version.

#### Notes

- Variants of the same job share container resources only if they happen to share a cache key. Because variants run concurrently in default and parallel modes, **avoid sharing cache keys across variants** unless they truly write disjoint paths.
- Matrix output (in default mode) lives in `.local-ci/logs/<timestamp>/<variant-name>.log`, same as parallel mode. Diagnostic logs go to `pipeline.log` in the same directory.

### Parallel Execution

By default, jobs run one after another. Local CI offers two parallel modes, controlled by mutually exclusive flags:

- `--parallel` (`-p`) — runs **all** selected jobs concurrently, ignoring stages
- `--parallel-stages` — runs **stages in order**, with the jobs inside each stage concurrently

```bash
# Run every job at once
local-ci run --parallel

# Run stages sequentially, jobs within a stage in parallel
local-ci run --parallel-stages
```

Passing both flags at once is rejected.

#### How It Works

Both modes share the same execution behavior:

1. **Concurrent Job Execution**:
   - Jobs in a parallel group run at the same time
   - With `--parallel`, every selected job forms a single group regardless of its stage
   - With `--parallel-stages`, each stage is its own group; stages run in the order declared under `stages`, and a stage whose jobs report a failure stops the pipeline before the next stage starts
   - Local CI waits for all jobs in a group to finish before proceeding

2. **Per-Job Log Files**:
   - Because concurrent jobs would interleave their output, logs are not streamed to the terminal
   - Each run creates a timestamped directory at `.local-ci/logs/<timestamp>/`
   - Every job writes its full output (image pull, container logs, job bootstrap/cleanup) to its own `<job-name>.log` file inside that directory
   - Diagnostic messages are collected in a separate `pipeline.log` file

3. **Live Status Board**:
   - In place of streamed logs, the terminal shows a live status board
   - Each job is listed with a spinner and its current state: `pending`, `running`, `passed`, or `failed`
   - The board repaints in place until all jobs complete; with `--parallel-stages`, a fresh board is shown per stage

4. **Error Aggregation**:
   - Errors from all jobs in a group are collected rather than failing on the first one
   - The pipeline reports a failure if any job failed, after every job in the group has finished

#### Notes

- The `.local-ci/` directory should be added to `.gitignore` so run logs are not committed.

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

Global variables are passed to the execution command alongside with os.Environ() for some additional QoL.

For job level bootstrap - job variables are passed instead

Timeout is specified as an integer representing minutes. If not provided, defaults to 5 minutes.

Example output:
```
Running bootstrap with timeout 5 minutes
Running bootstrap command: docker compose -f docker-compose.yml up -d
```

## Cleanup

Cleanup is the counterpart to bootstrap — it runs host-level teardown commands after the pipeline finishes. Cleanup runs regardless of whether the pipeline succeeded or failed, ensuring that infrastructure started during bootstrap is always torn down.

Shares the same variable passing logic with bootstrap.

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

## Job Bootstrap and Cleanup

In addition to global bootstrap and cleanup, Local CI supports per-job bootstrap and cleanup commands. These run on the host before and after each individual job, allowing jobs to set up and tear down their own infrastructure independently.

### When to Use

Use job-level bootstrap/cleanup when different jobs need different infrastructure. For example, a test job might need a database while a build job does not. Rather than spinning up everything in global bootstrap, each job can manage its own dependencies.

### How It Works

1. Before a job's container starts, `job_bootstrap` commands run sequentially on the host
2. The job executes in its container
3. After the job finishes (whether it succeeded or failed), `job_cleanup` commands run on the host
4. If job cleanup fails, the pipeline stops — leftover resources from a failed cleanup could affect subsequent jobs

### Key Differences from Global Bootstrap/Cleanup

| | Global Bootstrap/Cleanup | Job Bootstrap/Cleanup |
|---|---|---|
| **Scope** | Runs once for the entire pipeline | Runs per job that defines it |
| **When** | Before any jobs / after all jobs | Before/after each individual job |
| **Cleanup on failure** | Best-effort (logs errors, continues) | Fatal (stops the pipeline) |
| **Cleanup guarantee** | Always runs via `defer` | Always runs after job execution, regardless of job success/failure |

### Example Configuration

```yaml
stages:
  - build
  - test

Build:
  stage: build
  image: golang:1.22
  script:
    - go build ./...

Test:
  stage: test
  image: golang:1.22
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

In this example, the database is only started for the Test job and torn down immediately after, keeping the Build job lightweight.

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
   - Fixed one-hour timeout
   - Job-specific execution bypasses stage dependencies
   - No Github integration similar to GitLab

2. **Future Enhancements**:

   - Persistent services support
   - Git branch switching for cloned repositories via `--remote` command
   - Alias support for remote repository URLs
   - List command for available remote repositories and local clones
   - Installation via Homebrew
   - Remote execution support (run your pipeline on a remote machine)
