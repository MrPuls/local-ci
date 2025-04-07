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
   - [File System Handling](#file-system-handling)
   - [Caching System](#caching-system)
   - [Job-Specific Execution](#job-specific-execution)
   - [Stage-Specific Execution](#stage-specific-execution)
   - [Graceful Shutdown](#graceful-shutdown)
- [Limitations and Notes](#limitations-and-notes)

## Command Line Interface

Local CI features a simple but powerful command-line interface:

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

## Limitations and Notes

1. **Current Limitations**:
   - Single-node execution only
   - Sequential execution within stages
   - Fixed one-hour timeout
   - Job-specific execution bypasses stage dependencies

2. **Future Enhancements**:
   - Parallel job execution within stages
   - Persistent services support
   - Access to custom registries