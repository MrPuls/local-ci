# YAML Configuration Reference

## Table of Contents
- [Structure](#structure)
  - [Basic Structure](#basic-structure)
- [Configuration Fields](#configuration-fields)
  - [Pipeline Configuration](#pipeline-configuration)
    - [stages](#stages)
    - [variables (global level)](#variables-global-level)
    - [remote provider (global level)](#remote-provider-global-level)
    - [bootstrap](#bootstrap)
    - [cleanup](#cleanup)
    - [include](#include)
  - [Job Configuration](#job-configuration)
    - [stage](#stage)
    - [image](#image)
    - [workdir](#workdir)
    - [variables (job level)](#variables-job-level)
    - [network](#network)
    - [script](#script)
    - [cache](#cache)
    - [job_bootstrap](#job_bootstrap)
    - [job_cleanup](#job_cleanup)
    - [parallel](#parallel)
    - [matrix](#matrix)
    - [extends](#extends)
- [Variable Handling](#variable-handling)
- [Templates](#templates)
- [Network Configuration](#network-configuration)
- [Complete Example](#complete-example)

## Structure

The pipeline configuration uses YAML format and consists of two main parts:
1. Pipeline stages definition
2. Job configurations

### Basic Structure
```yaml
# Define stages and their order
stages:
  - step 1
  - step 2

# Global variables
variables:
  GLOBAL_KEY: value

# Job definitions
job_name:
  stage: step 1    # Must match one of the defined stages
  image: image_name
  workdir: /path   # Optional
  variables:       # Optional job-specific variables
    KEY: value
  network:         # Optional network configuration
    host_access: true
  cache:           # Optional caching configuration
    key: cache-key
    paths:
      - /cache/path
  job_bootstrap:   # Optional per-job host setup (requires job_cleanup if cleanup needed)
    run:
      - setup_command
    timeout: 5
  job_cleanup:     # Optional per-job host teardown (requires job_bootstrap)
    run:
      - teardown_command
    timeout: 5
  parallel: true   # Optional: detach this job from the sequential chain
  matrix:          # Optional: fan this job out into variants
    - VAR_A: [value1, value2]
      VAR_B: [value3, value4]
  script:
    - command1
    - command2
```

## Configuration Fields

### Pipeline Configuration

#### stages
- Required: Yes
- Type: Array of strings
- Description: Defines the stages of your pipeline and their execution order
- Example:
  ```yaml
  stages:
    - build
    - test
    - deploy
  ```

#### variables (global level)
- Required: No
- Type: Map of string key-value pairs
- Description: Environment variables available to all jobs. Job-specific variables with the same name will override global variables.
- Example:
  ```yaml
  variables:
    API_URL: "https://api.example.com"
    LOG_LEVEL: "info"
  ```

#### remote provider (global level)
- Required: No
- Type: Map of key-value pairs
- Description: Configuration for the remote provider (i.e. Gitlab, which is the only one supported. currently :c). Currently allows to get env variables from the remote provider. When collected, these variables are made available to all jobs as if they were defined in the `variables` (global) section. In case of a conflict, global variables take precedence over remote provider variables.
- Example:
  ```yaml
  remote_provider:
    url: "gitlab.example.com"
    project_id: 12345678
    access_token: "your_access_token"
  ```

#### Bootstrap
- Required: No
- Type: Map with the following options:
  - run: Array of strings (Shell commands executed on the host machine before any jobs are started)
  - timeout: Int (Maximum time to wait for bootstrap commands to complete in minutes, defaults to 5 if not set)
- Description: Defines host-level setup commands that run before any job containers are started. Intended for infrastructure preparation such as spinning up external services (e.g. via `docker compose`) that job containers will depend on. 

Important: Bootstrap runs on the host, not inside a container.
- Example
  ```yaml
  bootstrap:
    run:
      - docker compose -f docker-compose.yml up -d
    timeout: 5
  ```

#### Cleanup
- Required: No (requires bootstrap to be defined)
- Type: Map with the following options:
  - run: Array of strings (Shell commands executed on the host machine after all jobs have finished)
  - timeout: Int (Maximum time to wait for cleanup commands to complete in minutes, defaults to 5 if not set)
- Description: Defines host-level teardown commands that run after the pipeline finishes, regardless of whether the pipeline succeeded or failed. Intended for tearing down infrastructure started during bootstrap (e.g. stopping services via `docker compose`). Cleanup requires a bootstrap block to be defined — you can't clean up what wasn't set up.

Important: Cleanup runs on the host, not inside a container. Unlike bootstrap, cleanup is best-effort — if a command fails, the remaining commands still execute.
- Example
  ```yaml
  cleanup:
    run:
      - docker compose -f docker-compose.yml down
    timeout: 5
  ```

#### include
- Required: No
- Type: List of file paths (strings)
- Description: Pulls additional YAML files into the configuration. Each included file is parsed and its top-level keys (jobs, templates, global variables, stages, bootstrap/cleanup, remote provider) are merged into the main config. The main file always wins on conflicts; among multiple includes, later entries win over earlier ones. Includes can themselves contain `include:` directives — those are resolved recursively, with cycle detection.

  Paths are resolved **relative to the file that contains the `include:` directive**, not the working directory. Absolute paths are accepted as-is.

  Typical use: factor shared job templates, environment variables, or full stages into a `.ci/` directory and pull them into your main `.local-ci.yaml`. See [Templates](#templates) for the patterns this enables.
- Example:
  ```yaml
  include:
    - .ci/templates.yaml
    - .ci/jobs/test.yaml
  ```

### Job Configuration

#### stage
- Required: Yes
- Type: String
- Description: Specifies which stage the job belongs to. Must match one of the defined stages.
- Example:
  ```yaml
  job_name:
    stage: build
  ```

#### image
- Required: Yes
- Type: String
- Description: Docker image to use for this job
- Example:
  ```yaml
  job_name:
    image: alpine
  ```

#### workdir
- Required: No
- Type: String
- Description: Working directory inside the container
- Default: Root directory (/)
- Example:
  ```yaml
  job_name:
    workdir: /app
  ```

#### variables (job level)
- Required: No
- Type: Map of string key-value pairs
- Description: Environment variables available to the job. Overrides global variables with the same name.
- Example:
  ```yaml
  job_name:
    variables:
      API_KEY: secret
      DEBUG: "true"
  ```

#### network
- Required: No
- Type: Map with the following options:
  - host_access: boolean
    - Enables access to host machine services via 'host.docker.internal'
  - host_mode: boolean
    - Uses host network mode (not available on macOS)
- Description: Controls network configuration for the container
- Example:
  ```yaml
  job_name:
    network:
      host_access: true  # Access host via host.docker.internal
      # OR
      host_mode: true    # Use host network directly
  ```

#### script
- Required: Yes
- Type: Array of strings
- Description: Commands to execute in the container
- Example:
  ```yaml
  job_name:
    script:
      - echo "Running tests..."
      - go test ./...
  ```

#### cache
- Required: No
- Type: Map with the following options:
  - key: String
    - Used to uniquely identify the cache volume
  - paths: Array of strings
    - List of directory paths to be cached
- Description: Specifies paths to be cached and reused across runs using Docker volumes. Useful for dependencies, build artifacts, etc.
- Example:
  ```yaml
  job_name:
    cache:
      key: build-deps-v1
      paths:
        - "/.venv"          # Python virtual environment
        - "/node_modules"   # Node.js dependencies
        - "/build"          # Build artifacts
  ```

#### job_bootstrap
- Required: No
- Type: Map with the following options:
  - run: Array of strings (Shell commands executed on the host machine before the job starts)
  - timeout: Int (Maximum time to wait for bootstrap commands to complete in minutes, defaults to 5 if not set)
- Description: Defines host-level setup commands that run before this specific job's container is started. Unlike global bootstrap which runs once before the entire pipeline, job bootstrap runs before each individual job that defines it. If a job bootstrap command fails, the pipeline stops and the job does not execute.

Important: Job bootstrap runs on the host, not inside a container.
- Example
  ```yaml
  Build:
    stage: build
    image: alpine
    job_bootstrap:
      run:
        - docker compose -f docker-compose.test.yml up -d
      timeout: 3
    script:
      - echo "running tests"
  ```

#### job_cleanup
- Required: No (requires job_bootstrap to be defined)
- Type: Map with the following options:
  - run: Array of strings (Shell commands executed on the host machine after the job finishes)
  - timeout: Int (Maximum time to wait for cleanup commands to complete in minutes, defaults to 5 if not set)
- Description: Defines host-level teardown commands that run after this specific job finishes. Job cleanup runs regardless of whether the job succeeded or failed, ensuring that infrastructure started during job bootstrap is always torn down. If job cleanup fails, the pipeline stops — unlike global cleanup, job cleanup failure is treated as fatal because leftover resources could affect subsequent jobs.

Important: Job cleanup runs on the host, not inside a container. Requires job_bootstrap to be defined — you can't clean up what wasn't set up.
- Example
  ```yaml
  Build:
    stage: build
    image: alpine
    job_bootstrap:
      run:
        - docker compose -f docker-compose.test.yml up -d
      timeout: 3
    job_cleanup:
      run:
        - docker compose -f docker-compose.test.yml down
      timeout: 2
    script:
      - echo "running tests"
  ```

#### parallel
- Required: No
- Type: Boolean
- Default: `false`
- Description: When `true`, this job runs detached in the background at pipeline start, while other (non-parallel) jobs continue running sequentially. The pipeline waits for all detached jobs to finish before exiting, but a detached job's failure does not stop the sequential chain. Detached jobs write their output to per-job log files instead of streaming to stdout. Has no effect under `--parallel` or `--parallel-stages` — those flags have their own execution model and the keyword is ignored silently. See [Per-Job Parallel Flag](tech-reference.md#per-job-parallel-flag) for full execution details.
- Example:
  ```yaml
  Lint:
    stage: build
    image: golang:1.22
    parallel: true
    script:
      - go vet ./...
  ```

#### matrix
- Required: No
- Type: List of entries. Each entry is a map of variable name to value(s); values can be a scalar or a list of scalars.
- Description: Fans the job out into multiple variants — one per combination of variable values. Each variant runs as an independent job with the matrix values merged into its `variables`. Within a single entry, list values are expanded as the cartesian product. Multiple entries produce independent groups of variants, which is how you build asymmetric matrices (different variable sets per entry).

  Variants are automatically named `<JobName>_<key>.<value>_<key>.<value>...` with keys sorted alphabetically for determinism. This name is also used as the Docker container name, so matrix keys and values must match `[a-zA-Z0-9_.-]+` — unsafe characters are rejected at config load.

  By default, the variants of a single job run concurrently and the sequential pipeline waits for the whole group to finish (a "barrier"). Combining `matrix:` with `parallel: true` makes every variant detached individually — the pipeline does not wait between variants and the parent job's group runs in the background. Under `--parallel` and `--parallel-stages`, variants are just normal jobs that join the corresponding parallel pool. See [Matrix Builds](tech-reference.md#matrix-builds) for full execution semantics.
- Example (single entry, full cartesian product):
  ```yaml
  Test:
    stage: test
    image: golang:1.22
    matrix:
      - GO_VERSION: ["1.21", "1.22"]
        OS: [linux, alpine]
    script:
      - go test ./...
  ```
  Generates four variants: `Test_GO_VERSION.1.21_OS.alpine`, `Test_GO_VERSION.1.21_OS.linux`, `Test_GO_VERSION.1.22_OS.alpine`, `Test_GO_VERSION.1.22_OS.linux`.
- Example (multiple entries, asymmetric):
  ```yaml
  Deploy:
    stage: deploy
    image: alpine
    matrix:
      - PROVIDER: aws
        REGION: [us-east, us-west]
      - PROVIDER: ovh
        REGION: [eu-west]
    script:
      - ./deploy.sh
  ```
  Generates three variants: `Deploy_PROVIDER.aws_REGION.us-east`, `Deploy_PROVIDER.aws_REGION.us-west`, `Deploy_PROVIDER.ovh_REGION.eu-west`. The `ovh/us-east` combination is not generated because each entry is its own product.

Important: Variants of a job run concurrently in most modes, so avoid sharing a `cache.key` across them unless they write to truly disjoint paths — concurrent writes to the same Docker volume can corrupt the cache.

#### extends
- Required: No
- Type: Either a single string or a list of strings, each naming a template (or another job) defined elsewhere in the merged configuration.
- Description: Inherits fields from one or more templates. Templates are applied left-to-right, and the consuming job's own fields override any inherited values. Templates can themselves extend other templates — the chain is resolved recursively, with cycle detection.

  Most users reference templates declared with a leading dot (e.g. `.go-base`); those names are reserved for templates and are never executed as jobs. You can also extend a regular (non-template) job — it's just a name lookup in the merged config.

  See [Templates](#templates) for the full set of merge rules and patterns.
- Example (single template):
  ```yaml
  Build:
    extends: .go-base
    stage: build
    script:
      - go build ./...
  ```
- Example (multiple templates, later wins on conflict):
  ```yaml
  Test:
    extends:
      - .go-base
      - .with-db
    stage: test
    script:
      - go test ./...
  ```

## Variable Handling

Global variables and job-specific variables are merged, with job-specific variables taking precedence:

```yaml
variables:
  FOO: "BAR"  # Global variable
  
test_job:
  variables:
    FOO: "BAZ"  # Overrides the global value
    LOCAL: "VALUE"  # Job-specific variable
  script:
    - echo $FOO     # Outputs: BAZ
    - echo $LOCAL   # Outputs: VALUE
```

## Network Configuration

The network configuration allows containers to access services running on the host machine:

### Using host_access

```yaml
test_job:
  network:
    host_access: true
  script:
    - curl http://host.docker.internal:8080  # Access host service
```

### Using host_mode (Linux only)

```yaml
test_job:
  network:
    host_mode: true
  script:
    - curl http://localhost:8080  # Direct access to host services
```

## Templates

Templates let you factor out common job configuration once and reuse it across many jobs. A template is just a job-shaped block whose name starts with a dot (`.`). The leading dot marks it as "not a real job": it is parsed and made available as a target for `extends:`, but it is never executed.

### Defining a template

A template can live in the main config or in any included file. It looks exactly like a job, but its name begins with `.`:

```yaml
.go-base:
  image: golang:1.22
  workdir: /app
  cache:
    key: go-mod-cache
    paths:
      - /go/pkg/mod
  variables:
    GOFLAGS: "-count=1"
```

A template does not need to be complete (it can omit `stage`, `script`, etc.) — those fields are expected to come from the consuming job.

### Consuming a template

A job references one or more templates via `extends:`. Fields from templates flow into the job, and the job's own fields override them on conflict:

```yaml
Build:
  extends: .go-base
  stage: build
  script:
    - go build ./...
```

### Sharing templates across files

Put templates in a dedicated file and pull it in with `include:`. Paths resolve relative to the file containing the `include:` directive.

```yaml
# .ci/templates.yaml
.go-base:
  image: golang:1.22
  workdir: /app

.with-db:
  job_bootstrap:
    run:
      - docker compose -f docker-compose.test.yml up -d
    timeout: 3
  job_cleanup:
    run:
      - docker compose -f docker-compose.test.yml down
    timeout: 2
```

```yaml
# .local-ci.yaml
include:
  - .ci/templates.yaml

stages:
  - build
  - test

Build:
  extends: .go-base
  stage: build
  script:
    - go build ./...

Test:
  extends:
    - .go-base
    - .with-db
  stage: test
  script:
    - go test ./...
```

### Merge rules

How a field flows from template to job depends on its type:

| Field type | Rule |
|---|---|
| Scalars (`image`, `stage`, `workdir`) | Local non-empty value overrides template. |
| Lists (`script`, `cache.paths`, `matrix`, `run` arrays) | If the job declares the list at all, it **replaces** the template's list entirely. |
| `variables` map | Deep merged: keys from the template are inherited, keys defined locally override per key. |
| Nested objects (`cache`, `network`, `job_bootstrap`, `job_cleanup`) | Replaced atomically when the job defines its own. |
| `parallel` | Inherited from the template unless the job sets `parallel:` itself. A job can explicitly set `parallel: false` to opt out of a template that enabled it. |

### Multiple `extends:` ordering

When `extends:` is a list, templates are applied left-to-right. The rightmost template wins on conflicts, and the consuming job wins over all of them:

```yaml
.a:
  image: alpine
  variables:
    X: from-a

.b:
  image: ubuntu
  variables:
    X: from-b
    Y: only-in-b

Job:
  extends: [.a, .b]
  script:
    - echo $X $Y
# Effective image: ubuntu, X=from-b, Y=only-in-b
```

### Chained templates

Templates can themselves use `extends:`. The chain is resolved recursively, so you can build layered hierarchies:

```yaml
.base:
  image: alpine

.with-curl:
  extends: .base
  script:
    - apk add curl

Job:
  extends: .with-curl
  stage: build
# Resolved: image=alpine, script=[apk add curl], stage=build
```

Cycles are rejected at config load time.

### Naming conflicts

If two files (or the main file and an include) define a job or template with the same name, the main config wins. Among multiple includes, later entries in the `include:` list win over earlier ones. There is no merging across same-named entries — the loser is dropped entirely.

## Complete Example

```yaml
stages:
  - build
  - test

variables:
  GLOBAL_VAR: "shared across jobs"

bootstrap:
  run:
    - docker compose -f docker-compose.yml up -d
  timeout: 5

cleanup:
  run:
    - docker compose -f docker-compose.yml down
  timeout: 5

Build:
  stage: build
  image: node:16
  workdir: /app
  variables:
    NODE_ENV: production
  cache:
    key: node-modules
    paths:
      - /app/node_modules
  script:
    - npm install
    - npm run build

Lint:
  stage: build
  image: node:16
  workdir: /app
  parallel: true       # Run in background; do not block Test from starting
  script:
    - npm run lint

Test:
  stage: test
  image: node:16
  workdir: /app
  network:
    host_access: true
  cache:
    key: node-modules
    paths:
      - /app/node_modules
  job_bootstrap:
    run:
      - docker compose -f docker-compose.test.yml up -d
    timeout: 3
  job_cleanup:
    run:
      - docker compose -f docker-compose.test.yml down
    timeout: 2
  matrix:              # Fan out across Node versions
    - NODE_VERSION: ["16", "18", "20"]
  script:
    - npm test
    - curl http://host.docker.internal:3000/health
```
