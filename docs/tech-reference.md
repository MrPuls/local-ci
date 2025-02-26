# Technical Reference and CLI Usage

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

## Limitations and Notes

1. **Current Limitations**:
   - Single-node execution only
   - Sequential execution within stages
   - Fixed one-hour timeout

2. **Future Enhancements**:
   - Parallel job execution within stages
   - Integration with host network for testing against localhost
   - Persistent services support