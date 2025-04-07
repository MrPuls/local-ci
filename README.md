# Local CI

Local CI is a tool that allows you to run CI/CD pipelines locally using Docker containers. It helps developers test and debug their CI pipelines without pushing to remote repositories.

## Features

- Run CI pipeline jobs locally using Docker
- Stages-based execution similar to GitLab CI
- Configuration using YAML format
- Global and job-specific environment variables
- Working directory customization
- Cache support for dependencies and build artifacts
- Automatic file copying with .gitignore support
- Real-time log streaming from containers
- Automatic container cleanup
- Stage-based pipeline execution

## Installation

### Prerequisites

1. Go 1.21 or later
   - Visit [Go Downloads](https://golang.org/dl/) page
   - Download and install the latest version for your operating system
   - Verify installation with `go version`

2. Docker
   - Must be installed and running
   - Your user must have permissions to access the Docker daemon
   - Verify installation with `docker --version`

### Installing

```bash
go install github.com/MrPuls/local-ci/cmd/local-ci@latest
```

## Usage

### Check Installation

```bash
local-ci --version
```

### Run a Pipeline

```bash
# Run using default .local-ci.yaml in current directory
local-ci run

# Run with a specific config file using --config/-c flag
local-ci run --config my-pipeline.yaml

# Run a specific job using --job/-j
local-ci run --job JobName

# Or to run multiple jobs

local-ci run --job jobName1,JobName2

# Run jobs from a specific stage using --stage/-s

local-ci run --stage stageName

# Or use multiple stages
local-ci run --stage stageName1,stageName2
```

## Quick Start

1. Start Docker

2. Create a `.local-ci.yaml` file in your project root:

```yaml
stages:
  - build
  - test

variables:
  GLOBAL_VAR: global_value

Build:
  stage: build
  image: golang:1.21
  variables:
    GO_FLAGS: "-v"
  script:
    - echo "Building application..."
    - go build $GO_FLAGS
  cache:
    key: go-build-cache
    paths:
      - .go/
      - build/

Test:
  stage: test
  image: golang:1.21
  script:
    - echo "Testing with global var: $GLOBAL_VAR"
    - go test ./...
  cache:
    key: go-build-cache
    paths:
      - .go/
```

3. Run the pipeline:

```bash
local-ci run
```

## YAML Configuration

### Basic Structure

```yaml
# Define stages and their order
stages:
  - build
  - test
  - deploy

# Global variables (available to all jobs)
variables:
  GLOBAL_KEY: global_value

# Job definitions
job_name:
  stage: build    # Must match one of the defined stages
  image: image_name
  workdir: /path   # Optional
  variables:       # Job-specific variables (override globals)
    KEY: value
  script:
    - command1
    - command2
  cache:           # Optional cache configuration
    key: cache-key
    paths:
      - path/to/cache
      - another/path
```

## Documentation

- [YAML Configuration Reference](docs/yaml-reference.md)
- [Technical Reference](docs/tech-reference.md)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)