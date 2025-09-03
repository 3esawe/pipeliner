# URL/Value Replacement Feature

## Overview

The URL/Value Replacement feature allows tools in the pipeline to execute multiple times, once for each line in a dependency's output file. This is particularly useful for tools that don't accept file input but need to process each URL or value individually.

## Architecture

### Interface Hierarchy

The replacement feature uses a clean interface hierarchy:

```go
// Base command execution
type CommandRunner interface {
    Run(ctx context.Context, command string, args []string) error
}

// Extended command execution with replacement capabilities
type ReplacementCommandRunner interface {
    CommandRunner
    RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error
}
```

**Why ReplacementCommandRunner embeds CommandRunner:**
- **Liskov Substitution Principle**: A `ReplacementCommandRunner` can be used anywhere a `CommandRunner` is expected
- **Backward Compatibility**: Existing tools continue to work without modification
- **Optional Enhancement**: Tools with `replace` configuration automatically use replacement logic, others fall back to normal execution

### Dynamic Configuration Resolution

Instead of hardcoded filename patterns, the system uses:

1. **ToolRegistry**: Provides access to all tool configurations at runtime
2. **Dynamic Output Detection**: Automatically detects output files from tool configurations
3. **Fallback Patterns**: Uses sensible defaults when dynamic detection fails

## How It Works

When a tool configuration includes a `replace` field (and optionally `replace_from`), the pipeline will:

1. **Configuration Analysis**: Check if tool has `replace` token defined
2. **File Resolution**: 
   - Use `replace_from` if specified
   - Otherwise, dynamically resolve from dependency's output configuration
   - Fall back to `{dependency}_output.txt` pattern
3. **File Processing**: Read the resolved file line by line
4. **Command Execution**: For each non-empty, non-comment line:
   - Replace all instances of the replacement token with the current value
   - Execute the tool command with the replaced arguments

## Configuration

### Basic Configuration

```yaml
- name: ffuf
  description: Fuzzing tool for discovering hidden resources
  command: ffuf
  type: recon
  replace: "{{URL}}"                    # Token to be replaced
  replace_from: "httpx_output.txt"      # File to read values from (optional)
  depends_on: ["httpxbb"]
  flags:
    - flag: "-u"
      default: "{{URL}}/FUZZ"           # Token will be replaced here
    - flag: "-w"
      default: "/path/to/wordlist.txt"
    - flag: "-mc"
      default: "200,401-403"
```

### Configuration Fields

- `replace`: The token that will be replaced with each value from the file (e.g., `"{{URL}}"`)
- `replace_from`: (Optional) The file to read replacement values from. If not specified, it will be inferred from the first dependency's output file
- `depends_on`: Must specify at least one dependency whose output file contains the replacement values

### Auto-inference of Replacement Files

If `replace_from` is not specified, the system will attempt to infer the replacement file based on the first dependency:

- `httpx` or `httpxbb` → `httpx_output.txt`
- `subfinder` → `subfinder_output.txt`
- `findomain` → `subdomain_findomain_output.txt`
- `chaos-client` → `subdomain_chaos_client_output.txt`
- Other tools → `{toolname}_output.txt`

## Examples

### Example 1: Directory Fuzzing with ffuf

```yaml
- name: ffuf
  description: Directory fuzzing on discovered URLs
  command: ffuf
  type: recon
  replace: "{{URL}}"
  depends_on: ["httpxbb"]
  flags:
    - flag: "-u"
      default: "{{URL}}/FUZZ"
    - flag: "-w"
      default: "/usr/share/wordlists/dirb/common.txt"
    - flag: "-mc"
      default: "200,301,302,403"
    - flag: "-o"
      default: "ffuf_{{URL}}_output.json"    # URLs in filenames are also replaced
```

### Example 2: Custom URL Processing Tool

```yaml
- name: custom-scanner
  description: Custom security scanner
  command: ./custom-scanner
  type: vuln
  replace: "{{TARGET}}"
  replace_from: "live_urls.txt"
  depends_on: ["httpxbb"]
  flags:
    - flag: "--target"
      default: "{{TARGET}}"
    - flag: "--output"
      default: "scan_results.json"
    - flag: "--threads"
      default: "10"
```

## File Format Requirements

The replacement file should contain one value per line:
- Empty lines are ignored
- Lines starting with `#` are treated as comments and ignored
- Leading and trailing whitespace is trimmed

Example replacement file:
```
http://example.com
https://subdomain.example.com
http://another-domain.com
# This is a comment and will be ignored

http://final-url.com
```

## Implementation Details

### Architecture

1. **ReplacementCommandRunner**: Located in `pkg/runner/replacement.go`, handles the replacement logic
2. **Enhanced ToolConfig**: Updated in `pkg/tools/config.go` with new `replace` and `replace_from` fields
3. **ConfigurableTool Updates**: Modified in `pkg/tools/tool.go` to detect and handle replacement scenarios

### Error Handling

- If the replacement file doesn't exist or can't be read, the tool execution fails
- If individual command executions fail during replacement, errors are logged but execution continues with remaining values
- Missing replacement tokens in configuration result in descriptive error messages

### Performance Considerations

- Each replacement value results in a separate command execution
- Large replacement files may result in many individual tool executions
- Consider using appropriate concurrency limits and timeouts

## Migration Guide

### Existing Configurations

Existing tool configurations will continue to work without changes. The replacement feature is opt-in via the `replace` field.

### Converting Tools to Use Replacement

1. Add the `replace` field with your replacement token (e.g., `"{{URL}}"`)
2. Optionally add `replace_from` to specify the source file
3. Update flag defaults to include the replacement token where needed
4. Ensure the tool depends on another tool that produces the replacement values

Before:
```yaml
- name: tool
  command: tool
  depends_on: ["httpx"]
  flags:
    - flag: "-u"
      default: "httpx_output.txt"  # File input
```

After:
```yaml
- name: tool
  command: tool
  replace: "{{URL}}"               # Enable replacement
  depends_on: ["httpx"]
  flags:
    - flag: "-u"
      default: "{{URL}}"           # Individual URL input
```
