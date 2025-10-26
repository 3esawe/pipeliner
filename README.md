# Pipeliner

> ⚠️ **This is currently in beta!** Things might break, features might change. If you find bugs or have ideas, open an issue.

So basically I got tired of running the same recon commands over and over, waiting for subfinder to finish before running httpx, then nuclei, and so on. Pipeliner is my solution to that - it chains your security tools together in a pipeline you configure once and run whenever.

It's built in Go, uses YAML configs, and tries to be smart about running things in parallel when it can. Also has some nice extras like Discord notifications and a web UI for tracking scans.

## What it does

- **Run tools in sequence, parallel, or smart DAG mode** - You pick how things run
- **YAML configs** - Write once, run many times
- **Hook system** - Do things between stages (like combining outputs or sending notifications)
- **Discord bot** - Get pinged when your scans finish or find something
- **Periodic scans** - Set it and forget it, scans run every X hours
- **Web UI** - Track your scans, view results, see screenshots (beta feature)
- **Actually handles dependencies** - httpx won't run before subfinder finishes

## Getting started

You'll need:
- Go 1.23+ 
- Whatever security tools you want to run (subfinder, httpx, nuclei, etc.)

```bash
git clone https://github.com/3esawe/pipeliner.git
cd pipeliner
make build
```

That's it. Binary will be in `bin/pipeliner`.

## Quick examples

List what configs you have:
```bash
./bin/pipeliner list-configs
```

Run a basic scan:
```bash
./bin/pipeliner scan -m full_recon -d example.com
```

Run with custom timeout and periodic scanning:
```bash
./bin/pipeliner scan -m full_recon -d example.com \
  --timeout 2h \
  --periodic-hours 8 \
  --verbose
```

## How to configure it

Everything's in YAML files under `config/`. Here's what the structure looks like:

```yaml
description: "What this pipeline does"
execution_mode: "hybrid"  # sequential, concurrent, or hybrid

tools:
  - name: "subfinder"
    description: "Find subdomains"
    type: "domain_enum"       # Stage type
    command: "subfinder"      # What to actually run
    depends_on: []            # Empty = runs first
    timeout: 30m              # Optional
    retries: 3                # Optional
    
    flags:
      - flag: "-d"
        option: "Domain"      # Maps to the --domain flag you pass
        required: true
      - flag: "-o"
        default: "subfinder_output.txt"
    
    posthooks:
      - "NotifierHook"        # Run this after the tool finishes
```

### Execution modes explained

**Sequential** - Tools run one after another in order. Simple but slow.
```yaml
execution_mode: sequential
```

**Concurrent** - Everything runs at once. Fast but chaotic if tools depend on each other.
```yaml
execution_mode: concurrent
```

**Hybrid (recommended)** - Uses a DAG to figure out what can run in parallel while respecting dependencies. Best of both worlds.
```yaml
execution_mode: hybrid
tools:
  - name: subfinder
    type: domain_enum
    # runs immediately
    
  - name: httpx
    type: recon
    depends_on: ["subfinder"]
    # waits for subfinder to finish
```

### Tool types (stages)

The `type` field tells Pipeliner what stage a tool belongs to:

- `domain_enum` - Subdomain enumeration (subfinder, findomain, etc.)
  - Auto triggers `CombineOutput` hook when all domain enum tools finish
- `recon` - HTTP probing, port scanning (httpx, nmap, etc.)
- `fingerprint` - Screenshots, tech detection (gowitness, wappalyzer, etc.)
- `vuln` - Vulnerability scanning (nuclei, nikto, etc.)
  - Auto triggers `NotifierHook` when all vuln tools finish

## Hook system

Pipeliner has two types of hooks:

**Stage hooks** (automatic) - Run when ALL tools in a stage finish:
- `CombineOutput` - Runs after `domain_enum`, creates `httpx_input.txt` with all found subdomains
- `NotifierHook` - Runs after `vuln`, sends findings to Discord

**Post hooks** (you control) - Run after individual tools:
```yaml
tools:
  - name: httpx
    command: httpx
    posthooks:
      - "NotifierHook"  # Send notification when this specific tool finishes
```

Check available hooks:
```bash
./bin/pipeliner list-hooks
```

## Discord notifications

If you want to get pinged when scans finish or find vulns:

1. Make a Discord bot (Google it, takes 2 minutes)
2. Set the token:
   ```bash
   export DISCORD_TOKEN="your-bot-token"
   ```
3. Add hooks to your YAML (see above)

That's it. You'll get messages when things complete or when nuclei finds something.

## Web UI (Beta)

There's a web UI now for tracking scans. Start the server:

```bash
./bin/pipeliner serve
```

Then go to `http://localhost:8080`. You can:
- View all scans
- See real-time progress
- Check subdomain results with open ports, screenshots, vulns
- View directory fuzzing results

**Note:** The UI is pretty rough around the edges right now. Works but could be prettier.

## Example configs

Check the `config/` folder for examples. Here are a few patterns:

**Basic subdomain enum:**
```yaml
description: "Just find subdomains"
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
```

**Full recon pipeline:**
```yaml
description: "The whole shebang"
execution_mode: hybrid
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

  - name: httpx
    type: recon
    command: httpx
    depends_on: ["subfinder"]
    flags:
      - flag: "-l"
        default: "httpx_input.txt"
      - flag: "-o"
        default: "httpx_output.txt"

  - name: nuclei
    type: vuln
    command: nuclei
    depends_on: ["httpx"]
    flags:
      - flag: "-list"
        default: "httpx_output.txt"
      - flag: "-o"
        default: "nuclei_output.txt"
```

## Periodic scans

Want to run scans every X hours? Just add `--periodic-hours`:

```bash
./bin/pipeliner scan -m full_recon -d example.com --periodic-hours 6
```

Runs the scan, waits 6 hours, runs it again. Repeat forever until you Ctrl+C it.

Each scan gets its own timestamped directory in `scans/`.

## Common issues

**"Tool not found"** - Install the security tool (subfinder, httpx, etc.) and make sure it's in your PATH

**"Permission denied"** - Check file permissions or run with sudo if needed (not recommended though)

**Scan doesn't finish** - Check `--timeout`, might need to increase it

**Discord notifications not working** - Make sure `DISCORD_TOKEN` is set and the bot is in your server

Use `--verbose` to see what's actually happening:
```bash
./bin/pipeliner scan -m config-name -d example.com --verbose
```

## Commands reference

```bash
# Run a scan
./bin/pipeliner scan -m <module-name> -d <domain>

# List available configs
./bin/pipeliner list-configs

# See what hooks are available
./bin/pipeliner list-hooks

# Start the web UI
./bin/pipeliner serve

# Get help
./bin/pipeliner --help
```

**Flags:**
- `-m, --module` - Which YAML config to use (required)
- `-d, --domain` - Target domain (required)
- `--timeout` - How long to wait before giving up (default: 30m)
- `--periodic-hours` - Run every X hours (default: 5)
- `--verbose` - Show debug logs
- `--config` - Path to config directory (default: ./config)

## Project structure

```
pipeliner/
├── bin/              # Compiled binary goes here
├── cmd/pipeliner/    # Main entry point
├── config/           # Your YAML configs
├── internal/         # Internal packages (services, db, etc.)
├── pkg/              # Core engine and tools
├── scans/            # Scan outputs (timestamped directories)
├── templates/        # Web UI templates
└── static/           # CSS and stuff
```

Scan output directories look like:
```
scans/full_recon_example.com_2025-08-24_18-03-30/
├── subfinder_output.txt
├── httpx_output.txt
├── httpx_input.txt
├── nuclei_output.txt
└── screenshots/
```

## Contributing

If you want to contribute or have ideas, open an issue or PR. The code is probably not perfect - I built this to scratch my own itch.

## Things I want to add

- [ ] Better web UI (it's functional but ugly)
- [ ] More hooks and customization
- [ ] Result diffing between scans
- [ ] Better error recovery
- [ ] Maybe a plugin system?
- [ ] Cloud storage for scan results
- [ ] Better screenshot handling

Built because I was tired of manual recon workflows. Hope it saves you some time too.
