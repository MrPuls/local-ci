# CLI Usage and Technical Reference

## Command Line Interface

The tool uses a simple CLI interface that runs pipeline configurations:

```bash
local-ci run
```

By default, the tool looks for a `.local-ci.yaml` file in the current directory.

## How It Works

The tool operates in several steps:

### 1. Configuration Loading
- Reads `.local-ci.yaml` from the current working directory, 
- Parses and validates the YAML configuration
- Maps the YAML to internal configuration structures

### 2. Pipeline Execution
For each job block in the configuration:

1. Container Setup:
    - Pulls the specified Docker image
    - Creates a container with:
        - Working directory as specified (defaults to "/")
        - Environment variables from the configuration
        - Shell command created from script array

2. File System Handling:
    - Creates a tar archive of the current working directory
    - Copies project files into the container

3. Execution:
    - Starts the container
    - Streams logs to stdout
    - Waits for container completion

4. Cleanup:
    - Removes the container after execution

### Technical Details

#### Container Configuration
- Uses Docker SDK for Go
- One-hour timeout per container
- Scripts are joined with '&&' to ensure sequential execution
- All container operations are context-aware with proper cleanup

#### Environment GlobalVariables
Environment variables specified in the configuration are passed directly to the container:
```yaml
variables:
  KEY: value
```
Becomes:
```
KEY=value
```
in the container environment.

#### File System Handling
The tool provides smart file system handling with .gitignore support:

1. File Collection:
    - Reads `.gitignore` if present in the project root
    - Respects all non-comment patterns from `.gitignore`
    - Walks through the project directory recursively
    - Skips files matching ignore patterns
    - Creates a tar archive containing all relevant files

2. Gitignore Processing:
    - Automatically detects and reads `.gitignore` file
    - Strips comments and empty lines
    - Applies patterns to both files and directories
    - Skips entire directories if they match ignore patterns

3. Archive Creation:
    - Creates a tar archive in memory
    - Maintains relative path structure
    - Preserves file metadata (permissions, timestamps)
    - Handles both regular files and directories

4. Container Integration:
    - Copies the created tar archive into the container
    - Extracts at the specified working directory
    - Maintains original file structure and permissions
