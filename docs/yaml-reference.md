# YAML Configuration Reference

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

# Job definitions
job_name:
  stage: step 1    # Must match one of the defined stages
  image: image_name
  workdir: /path   # Optional
  variables:       # Optional
    KEY: value
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

#### variables
- Required: No
- Type: Map of string key-value pairs
- Description: Environment variables available to the job
- Example:
  ```yaml
  job_name:
    variables:
      API_KEY: secret
      DEBUG: "true"
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

## Complete Example

```yaml
stages:
  - step 1
  - step 2

Test:
  stage: step 1
  image: alpine
  variables:
    FOO: BAR
    BAZ: EGGS
  script:
    - echo "Hello World"
    - echo $FOO
    - touch foo.txt
    - sleep 5
    - echo "Hello from txt file" >> foo.txt
    - echo $BAZ >> foo.txt
    - cat foo.txt
```