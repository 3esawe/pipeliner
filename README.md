# Pipeliner

**Pipeliner** is a modular, high-performance security scanning pipeline tool designed for reconnaissance and vulnerability assessment. It orchestrates multiple security tools in a configurable, scalable manner with support for different execution strategies and intelligent hook systems.

## 🚀 Features

- **Multiple Execution Strategies**: Sequential, Concurrent, and Hybrid (DAG-based) execution
- **Configurable Pipelines**: YAML-based configuration for maximum flexibility
- **Hook System**: Stage Hooks (system-controlled) and PostHooks (user-defined)
- **Discord Integration**: Real-time notifications for scan results
- **Periodic Scanning**: Automated recurring scans with configurable intervals
- **Dependency Management**: Smart tool orchestration based on dependencies
- **Progress Monitoring**: Real-time progress tracking and logging
- **Graceful Shutdown**: Proper signal handling and resource cleanup

## 📦 Installation

### Prerequisites

- Go 1.23.0 or later
- Security tools you want to orchestrate (subfinder, httpx, nuclei, etc.)

### Build from Source

```bash
git clone <repository-url>
cd pipeliner
make build
```

### Binary Usage

```bash
./bin/pipeliner --help
```

## 🎯 Quick Start

### 1. List Available Configurations

```bash
./bin/pipeliner list-configs
```

### 2. Run a Basic Scan

```bash
./bin/pipeliner scan -m full_recon -d example.com
```

### 3. Run with Custom Options

```bash
./bin/pipeliner scan -m full_recon -d example.com \
  --timeout 2h \
  --periodic-hours 8 \
  --verbose
```

## 📖 Command Reference

### Global Commands

| Command | Description |
|---------|-------------|
| `scan` | Execute a pipeline module against a target domain |
| `list-configs` | Display all available configuration files and descriptions |
| `list-hooks` | Show available hooks for YAML configurations |
| `completion` | Generate shell autocompletion scripts |
| `help` | Display help information |

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `./config` | Configuration directory path |
| `--periodic-hours` | int | `5` | Hours between periodic scans |
| `--timeout` | duration | `30m` | Global timeout for operations |
| `-v, --verbose` | bool | `false` | Enable verbose logging |

### Scan Command Flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `-m, --module` | string | ✅ | Pipeline module to execute |
| `-d, --domain` | string | ✅ | Target domain for scanning |

## ⚙️ Configuration

### YAML Configuration Structure

Pipeliner uses YAML files to define scanning pipelines. Here's the complete structure:

```yaml
# Optional description displayed in list-configs
description: "Pipeline description"

# Execution strategy: sequential, concurrent, or hybrid
execution_mode: "hybrid"

# List of tools to execute
tools:
  - name: "tool-name"              # Unique identifier
    description: "Tool description" # Optional description
    type: "tool-type"              # Stage type (see Tool Types)
    command: "command-name"        # Executable command
    depends_on: ["tool1", "tool2"] # Optional dependencies (hybrid mode)
    timeout: 30m                   # Optional tool-specific timeout
    retries: 3                     # Optional retry count
    
    # Command flags configuration
    flags:
      - flag: "--flag-name"        # Flag name
        option: "OptionName"       # Maps to Options field
        required: true             # Whether flag is required
        default: "value"           # Default value
        is_boolean: false          # Boolean flag (no value)
        is_positional: false       # Positional argument
    
    # User-defined hooks (optional)
    posthooks:
      - "NotifierHook"             # Hook name from list-hooks
```

### Tool Types (Stages)

Pipeliner organizes tools into logical stages:

| Stage | Type Value | Description | Stage Hook |
|-------|------------|-------------|------------|
| **Domain Enumeration** | `domain_enum` | Subdomain discovery tools | `CombineOutput` |
| **Reconnaissance** | `recon` | HTTP probing, service discovery | - |
| **Fingerprinting** | `fingerprint` | Technology detection, screenshots | - |
| **Vulnerability Scanning** | `vuln` | Security assessment tools | `NotifierHook` |

### Execution Strategies

#### 1. Sequential Strategy

Executes tools one after another in the order defined in the configuration.

**Characteristics:**
- **Execution Order**: Tools run in YAML definition order
- **Resource Usage**: Low (one tool at a time)
- **Completion Time**: Longest (sum of all tool times)
- **Dependencies**: Not supported (order-based)
- **Best For**: Simple pipelines, resource-constrained environments

**Example:**
```yaml
execution_mode: sequential
tools:
  - name: subfinder
    # runs first
  - name: httpx  
    # runs after subfinder completes
  - name: nuclei
    # runs after httpx completes
```

#### 2. Concurrent Strategy

Executes all tools simultaneously with no dependency consideration.

**Characteristics:**
- **Execution Order**: All tools start simultaneously
- **Resource Usage**: High (all tools running)
- **Completion Time**: Fastest (longest single tool time)
- **Dependencies**: Not supported (parallel execution)
- **Best For**: Independent tools, high-resource environments

**Example:**
```yaml
execution_mode: concurrent
tools:
  - name: subfinder
    # starts immediately
  - name: chaos-client
    # starts immediately
  - name: findomain
    # starts immediately
```

**⚠️ Note**: Tools should not depend on each other's output in concurrent mode.

#### 3. Hybrid Strategy (Recommended)

Uses Directed Acyclic Graph (DAG) to respect dependencies while maximizing parallelism.

**Characteristics:**
- **Execution Order**: Dependency-aware with maximum parallelism
- **Resource Usage**: Optimized (parallel where possible)
- **Completion Time**: Optimized (respects dependencies)
- **Dependencies**: Full support via `depends_on`
- **Best For**: Complex pipelines with tool interdependencies

**Example:**
```yaml
execution_mode: hybrid
tools:
  - name: subfinder
    type: domain_enum
    # no dependencies - starts immediately
    
  - name: findomain  
    type: domain_enum
    # no dependencies - starts with subfinder
    
  - name: httpx
    type: recon
    depends_on: ["subfinder", "findomain"]
    # waits for both domain enum tools
    
  - name: nuclei
    type: vuln
    depends_on: ["httpx"]
    # waits for httpx to complete
```

**DAG Execution Flow:**
```
subfinder ──┐
            ├─→ httpx ──→ nuclei
findomain ──┘
```

## 🔗 Hook System

Pipeliner features a sophisticated two-tier hook system:

### Stage Hooks (System-Controlled)

Automatically triggered when **ALL** tools in a stage complete.

| Hook | Stage | Purpose |
|------|--------|---------|
| `CombineOutput` | `domain_enum` | Combines all subdomain results into `httpx_input.txt` |
| `NotifierHook` | `vuln` | Sends notifications for vulnerability findings |

**Execution Example:**
```
subfinder (domain_enum) ──┐
                         ├─→ Stage Hook: CombineOutput
findomain (domain_enum) ──┘
                           ↓
                         httpx (recon)
                           ↓
                         nuclei (vuln) ──→ Stage Hook: NotifierHook
```

### PostHooks (User-Controlled)

Defined in YAML configurations, executed after **EACH** individual tool completes.

**Available PostHooks:**
- `NotifierHook`: Send Discord notifications for tool output

**YAML Usage:**
```yaml
tools:
  - name: httpx
    command: httpx
    posthooks:
      - "NotifierHook"  # Runs after httpx completes
```


### Hook Execution Timeline

```
Tool Execution → PostHooks → Stage Check → Stage Hooks (if stage complete)
```

## 🔔 Discord Integration

Enable real-time notifications for scan results.

### Setup

1. Create a Discord application and bot
2. Set the environment variable:
   ```bash
   export DISCORD_TOKEN="your-bot-token"
   ```

3. Configure hooks in your YAML:
   ```yaml
   tools:
     - name: nuclei
       posthooks:
         - "NotifierHook"
   ```

### Notification Types

- **Individual Tool Results**: Via PostHooks
- **Stage Completion**: Via Stage Hooks
- **Vulnerability Findings**: Automatic for `vuln` stage tools

## 📊 Monitoring and Logging

### Logging Levels

| Flag | Level | Output |
|------|-------|--------|
| Default | INFO | Basic execution flow |
| `--verbose` | DEBUG | Detailed execution information |

### Progress Tracking

Pipeliner provides real-time progress updates:

```
INFO[0000] Executing tools in hybrid (DAG-based)
INFO[0000] Hybrid DAG workers: 4
INFO[0001] Tool subfinder completed successfully
INFO[0002] Tool findomain completed successfully  
INFO[0002] Stage domain_enum completed. Triggering stage hooks...
INFO[0002] Stage hook combine_output completed successfully
INFO[0003] Tool httpx completed successfully
```

### Output Management

- **Working Directory**: Each scan creates a timestamped directory
- **Tool Outputs**: Individual files per tool
- **Combined Results**: Stage hooks create consolidated files
- **Logs**: Structured logging with timestamps and context

## 🔄 Periodic Scanning

Configure automated recurring scans:

```bash
# Scan every 8 hours
./bin/pipeliner scan -m full_recon -d example.com --periodic-hours 8
```

**Features:**
- **Configurable Intervals**: Any number of hours
- **Persistent Execution**: Continues until manually stopped
- **Directory Management**: New timestamped directory per scan
- **Graceful Shutdown**: SIGINT/SIGTERM handling

## 📁 Directory Structure

```
pipeliner/
├── bin/                    # Built binaries
├── cmd/pipeliner/         # Application entry point
├── config/                # YAML configuration files
│   ├── full_recon.yaml    # Complete recon pipeline
│   ├── subdomain.yaml     # Basic subdomain enumeration
│   └── ...
├── internal/              # Internal packages
│   ├── notification/      # Discord integration
│   └── utils/             # Utility functions
├── pkg/                   # Public packages
│   ├── engine/            # Core execution engine
│   ├── hooks/             # Hook implementations
│   ├── logger/            # Structured logging
│   ├── tools/             # Tool orchestration
│   └── ...
├── scans/                 # Scan output directories
│   └── full_recon_example.com_2025-08-24_18-03-30/
└── Makefile              # Build automation
```

## 🛠️ Configuration Examples

### Basic Subdomain Enumeration

```yaml
description: "Basic subdomain discovery"
execution_mode: concurrent
tools:
  - name: subfinder
    type: domain_enum
    command: subfinder
    flags:
      - flag: "-d"
        option: "Domain"
        required: true
      - flag: "-o"
        default: "subfinder_output.txt"

  - name: findomain
    type: domain_enum
    command: findomain
    flags:
      - flag: "-t"
        option: "Domain"
        required: true
      - flag: "-u"
        default: "findomain_output.txt"
```

### Complete Reconnaissance Pipeline

```yaml
description: "Full recon with vulnerability scanning"
execution_mode: hybrid
tools:
  # Stage 1: Domain Enumeration (Parallel)
  - name: subfinder
    type: domain_enum
    command: subfinder
    flags:
      - flag: "-d"
        option: "Domain"
        required: true
      - flag: "-o"
        default: "subfinder_output.txt"

  - name: findomain
    type: domain_enum
    command: findomain
    flags:
      - flag: "-t"
        option: "Domain"
        required: true
      - flag: "-u"
        default: "findomain_output.txt"

  # Stage 2: HTTP Probing (After domain enumeration)
  - name: httpx
    type: recon
    command: httpx
    depends_on: ["subfinder", "findomain"]
    flags:
      - flag: "-l"
        default: "httpx_input.txt"  # Created by CombineOutput hook
      - flag: "-o"
        default: "httpx_output.txt"
    posthooks:
      - "NotifierHook"  # Notify when HTTP probing completes

  # Stage 3: Vulnerability Scanning (After HTTP probing)
  - name: nuclei
    type: vuln
    command: nuclei
    depends_on: ["httpx"]
    flags:
      - flag: "-list"
        default: "httpx_output.txt"
      - flag: "-o"
        default: "nuclei_output.txt"
      - flag: "-s"
        default: "high,critical"
```

## 🚨 Error Handling

### Tool Failures

- **Sequential**: Pipeline stops on first failure
- **Concurrent**: Collects all failures, reports at end
- **Hybrid**: Failed tools skip dependents, continues with independent tools

### Hook Failures

- **PostHooks**: Tool marked as failed, pipeline may stop
- **Stage Hooks**: Logged but pipeline continues
- **Graceful Degradation**: Core functionality preserved

### Timeout Handling

```bash
# Global timeout for all operations
--timeout 2h

# Per-tool timeout in YAML
tools:
  - name: slow-tool
    timeout: 45m
```

## 🔧 Troubleshooting

### Common Issues

1. **Tool Not Found**
   ```
   ERROR: execution failed: executable not found
   ```
   **Solution**: Ensure security tools are installed and in PATH

2. **Permission Denied**
   ```
   ERROR: permission denied
   ```
   **Solution**: Check file permissions and user privileges

3. **Hook Not Found**
   ```
   WARN: Post hook SomeHook not found
   ```
   **Solution**: Use `./bin/pipeliner list-hooks` to see available hooks

4. **Configuration Errors**
   ```
   ERROR: failed to prepare scan: invalid options
   ```
   **Solution**: Validate YAML syntax and required fields

### Debug Mode

Enable verbose logging for detailed troubleshooting:

```bash
./bin/pipeliner scan -m config-name -d target.com --verbose
```

### Validation

Check configurations before running:

```bash
# List available configurations
./bin/pipeliner list-configs

# Validate specific config by attempting dry run
./bin/pipeliner scan -m config-name -d example.com --timeout 1s
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone repository
git clone <repository-url>
cd pipeliner

# Install dependencies
go mod download

# Build and test
make build
go test ./...

# Run locally
./bin/pipeliner scan -m subdomain -d example.com --verbose
```

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [Viper](https://github.com/spf13/viper) for configuration management
- [Logrus](https://github.com/sirupsen/logrus) for structured logging
- [DiscordGo](https://github.com/bwmarrin/discordgo) for Discord integration

## 📋 Roadmap

- [ ] Web UI for pipeline management
- [ ] Plugin system for custom tools
- [ ] Cloud deployment templates
- [ ] Advanced scheduling (cron-like)
- [ ] Result correlation and analysis
- [ ] Multi-target batch processing
- [ ] Custom hook development SDK

---

**Happy Scanning!** 🔍✨
