# FlowJS-Works Engine (Runner)

A Go-based workflow execution engine that interprets JSON DSL definitions to execute integration processes.

## Overview

The Runner is a lightweight, modular microservice that executes workflow processes defined in a JSON DSL (Domain Specific Language). It's designed to be extensible, with clean error handling and built-in audit logging via NATS.

## Architecture

```
services/engine/
├── cmd/runner/          # Main entry point
│   └── main.go         
├── internal/
│   ├── activities/      # Activity implementations
│   │   ├── activity.go  # Interface and registry
│   │   ├── logger.go    # Logger activity
│   │   └── http.go      # HTTP activity
│   ├── engine/          # Core execution engine
│   │   └── executor.go  # ProcessExecutor
│   └── models/          # Data structures
│       ├── process.go   # Process, Node, Trigger models
│       └── context.go   # Execution context with JSONPath
└── bin/                 # Compiled binaries
```

## Features

### Core Capabilities
- **JSON DSL Parser**: Parses and validates workflow definitions
- **Sequential Execution**: Executes nodes in order (parallel execution can be added later)
- **Execution Context**: Maintains state during execution with JSONPath-like access
- **Activity Registry**: Extensible system for adding new activity types

### Built-in Activities
- **Logger**: Logs messages to console with configurable levels
- **HTTP**: Makes HTTP requests with retry support
- **Script (script_ts)**: Executes JavaScript code for data transformations

### Error Handling
- Robust error handling at every level
- Configurable retry policies per node (max attempts, intervals)
- Process stops on first error (as per specification)
- Detailed error logging

### Audit Logging
- Asynchronous audit messages sent to NATS after each node execution
- Includes execution ID, node ID, status, output, and errors
- Subject: `audit.logs`

### Context & Data Flow
Supports simplified JSONPath syntax for data access:
- `$.trigger.body` - Access trigger payload
- `$.trigger.headers.date` - Access nested trigger data
- `$.nodes.nodeId.output` - Access output from previous nodes
- `$.nodes.nodeId.status` - Check node execution status

## Usage

### Building

```bash
cd services/engine
go build -o bin/runner ./cmd/runner
```

### Running

#### With embedded example:
```bash
./bin/runner -nats="nats://localhost:4222"
```

#### With custom process file:
```bash
./bin/runner -process=my-process.json -nats="nats://localhost:4222"
```

#### With custom trigger data:
```bash
./bin/runner -process=my-process.json -trigger=my-trigger.json -nats="nats://localhost:4222"
```

#### Without NATS (audit logging disabled):
```bash
./bin/runner -process=my-process.json -nats=""
```

### Command Line Options

- `-process`: Path to the process JSON file (optional, uses embedded example if not provided)
- `-trigger`: Path to the trigger data JSON file (optional, uses default trigger data if not provided)
- `-nats`: NATS server URL for audit logging (default: "nats://localhost:4222", set to "" to disable)

## Process Definition (DSL)

### Basic Structure

```json
{
  "definition": {
    "id": "process-id",
    "version": "1.0.0",
    "name": "Process Name",
    "description": "Description",
    "settings": {
      "persistence": "full",
      "timeout": 30000,
      "error_strategy": "stop_and_rollback"
    }
  },
  "trigger": {
    "id": "trg_01",
    "type": "http_webhook",
    "config": {
      "path": "/v1/endpoint",
      "method": "POST"
    }
  },
  "nodes": [
    {
      "id": "node1",
      "type": "logger",
      "description": "Description",
      "input_mapping": {
        "message": "$.trigger.body"
      },
      "config": {
        "level": "info"
      }
    }
  ],
  "transitions": []
}
```

### Node Types

#### Logger
Logs messages to console.

```json
{
  "id": "log_node",
  "type": "logger",
  "input_mapping": {
    "message": "$.trigger.body"
  },
  "config": {
    "level": "info"
  }
}
```

#### HTTP
Makes HTTP requests.

```json
{
  "id": "api_call",
  "type": "http",
  "input_mapping": {
    "body": "$.trigger.body",
    "headers": {
      "Authorization": "Bearer token"
    }
  },
  "config": {
    "url": "https://api.example.com/endpoint",
    "method": "POST",
    "timeout": 30
  },
  "retry_policy": {
    "max_attempts": 3,
    "interval": "2s",
    "type": "exponential"
  }
}
```

#### Script (script_ts)
Executes JavaScript code for data transformations using the Goja JavaScript engine.

**Features:**
- Access input data via the `input` object
- Transform, filter, and manipulate data using JavaScript
- Return objects, arrays, or primitive values
- Comprehensive error handling for syntax and runtime errors

**Example 1: Simple transformation**
```json
{
  "id": "transform_data",
  "type": "script_ts",
  "description": "Transform user data",
  "input_mapping": {
    "name": "$.trigger.body.name",
    "age": "$.trigger.body.age"
  },
  "script": "const output = { greeting: 'Hello, ' + input.name + '!', isAdult: input.age >= 18, processedAt: new Date().toISOString() }; output;"
}
```

**Example 2: Array operations**
```json
{
  "id": "filter_users",
  "type": "script_ts",
  "description": "Filter adult users",
  "input_mapping": {
    "users": "$.trigger.body.users",
    "companyName": "$.trigger.body.company"
  },
  "script": "const adults = input.users.filter(u => u.age >= 18); const output = { company: input.companyName, adultCount: adults.length, adultNames: adults.map(u => u.name) }; output;"
}
```

**Security Note:** The script runs in a sandboxed Goja environment and has no access to file system, network, or system resources.

### Input Mapping

Input mapping allows you to extract data from the execution context:

```json
"input_mapping": {
  "email": "$.trigger.body.email",
  "userId": "$.trigger.body.user_id",
  "previousOutput": "$.nodes.previous_node.output"
}
```

## Testing

### Example Processes

The repository includes several example processes:

1. **Embedded Example** (`cmd/runner/main.go`): Simple hello world process
2. **test-process.json**: Tests context and JSONPath functionality
3. **test-script-process.json**: Demonstrates JavaScript transformation with script_ts
4. **error-test.json**: Tests error handling and retry logic
5. **test-catfact-process.json**: External GET integration flow against catfact.ninja

### Running Tests

```bash
# Start NATS for audit logging
docker compose up -d nats

# Test with embedded example
./bin/runner

# Test with custom process
./bin/runner -process=test-process.json -trigger=test-trigger.json

# Test error handling
./bin/runner -process=error-test.json

# Test external catfact GET flow
./bin/runner -process=test-catfact-process.json -trigger=test-catfact-trigger.json
```

## Development

### Adding New Activities

1. Create a new file in `internal/activities/`
2. Implement the `Activity` interface:

```go
type MyActivity struct{}

func (a *MyActivity) Name() string {
    return "my_activity"
}

func (a *MyActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
    // Implementation
    return output, nil
}
```

3. Register it in `activity.go`:

```go
func NewActivityRegistry() *ActivityRegistry {
    registry := &ActivityRegistry{
        activities: make(map[string]Activity),
    }
    registry.Register(&LoggerActivity{})
    registry.Register(&HTTPActivity{})
    registry.Register(&MyActivity{})  // Add your activity
    return registry
}
```

## Dependencies

- `github.com/google/uuid` - For generating execution IDs
- `github.com/nats-io/nats.go` - For NATS messaging

## Future Enhancements

- Parallel node execution (fork/join)
- Conditional transitions based on node outputs
- More built-in activities (SQL, file operations, etc.)
- JavaScript/TypeScript scripting support for transformations
- Hot-reload of process definitions
- Graceful shutdown handling
- Metrics and monitoring
- State persistence to database
