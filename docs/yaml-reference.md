# YAML Configuration Reference

## Table of Contents
- [Structure](#structure)
  - [Basic Structure](#basic-structure)
- [Configuration Fields](#configuration-fields)
  - [Pipeline Configuration](#pipeline-configuration)
    - [stages](#stages)
    - [variables (global level)](#variables-global-level)
  - [Job Configuration](#job-configuration)
    - [stage](#stage)
    - [image](#image)
    - [workdir](#workdir)
    - [variables (job level)](#variables-job-level)
    - [network](#network)
    - [script](#script)
    - [cache](#cache)
- [Variable Handling](#variable-handling)
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

## Complete Example

```yaml
stages:
  - build
  - test

variables:
  GLOBAL_VAR: "shared across jobs"

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
  script:
    - npm test
    - curl http://host.docker.internal:3000/health
```