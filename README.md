# Local CI 

Local CI  is a tool that allows you to run CI/CD pipelines locally using Docker containers. It helps developers test and debug their CI pipelines without pushing to remote repositories.

## Features

- Run CI pipeline jobs locally using Docker
- Configuration using YAML format
- Environment variable support
- Working directory customization
- Automatic file copying with .gitignore support
- Real-time log streaming from containers
- Automatic container cleanup

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
go install github.com/MrPuls/local-ci@latest
```

## Quick Start

1. Start Docker

2. Create a `.local-ci.yaml` file in your project root:

```yaml
stages:
  - test

variables:
   BAR: BAZ

Test:
  stage: test
  image: alpine
  variables:
    FOO: BAR
  script:
    - echo "Hello World"
    - echo $FOO
    - echo $BAR
```

3. Run the pipeline:

```bash
local-ci run
```

## Documentation

- [YAML Configuration Reference](docs/yaml-reference.md)
- [Technical Reference](docs/tech-reference.md)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
