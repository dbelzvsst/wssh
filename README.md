# wssh

`wssh` is an iTerm2 SSH orchestrator for macOS that allows you to quickly connect to hosts using custom layouts and macros, with support for session logging and SSH key management. 

## Requirements

- macOS (with iTerm2 installed)
- Go 1.25+ (https://golang.org/dl/)
- SSH keys for your target hosts
- (Optional) `~/.wssh.yaml` configuration file (auto-generated if missing)

### Go Dependencies
- `github.com/spf13/cobra`
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `gopkg.in/yaml.v3`

All dependencies are managed via Go modules.

## Build & Installation Instructions

1. Clone this repository:
   ```sh
   git clone <repo-url>
   cd wssh

```

2. Build the application:
```sh
go build -o wssh .

```


3. Install it globally (recommended):
```sh
sudo mv wssh /usr/local/bin/

```


*This allows you to run `wssh` from any directory and ensures iTerm2 can find it for background macros.*

## Usage

### Initial Setup

* On first run, `wssh` will generate a default `~/.wssh.yaml` config file if it does not exist.
* Edit `~/.wssh.yaml` to customize hosts, groups, layouts, and macros as needed.

### Running wssh

* **Interactive TUI:**
```sh
wssh

```


Launches a terminal UI to select hosts and layouts interactively.
* **Direct CLI:**
```sh
wssh <host-alias> [layout]

```


Connects directly to the specified host using the chosen layout (e.g., `single`, `2h`, `2v`, `3h`, `3v`, `4g`).
You can also pass connection flags:

- `--retry` (or `-r`): retry failed SSH connections every 5 seconds until success or `Ctrl+C`
- `--local`: run SSH in the current terminal instead of opening a new iTerm tab

Examples:

```sh
wssh my-host --retry
wssh my-host --local
wssh my-host --local --retry
```

* **Reconnect Last Host:**
```sh
wssh last [layout]
wssh --last [layout]

```


Both forms reconnect to the last host recorded in `~/.wssh_history`, with an optional layout (for example `wssh last 4g` or `wssh --last 4g`). Configured host options are preserved, and ad-hoc hosts also work. These commands use the default `single` layout when none is given and report an error if there is no history yet. History records connection launches, not verified successful SSH logins.

The same connection flags apply: `wssh --last --local` uses the current terminal, and `wssh last --retry` retries failed connections.
* **Init Local Workspace:**
```sh
wssh init

```


Opens a new iTerm tab, splits vertically, then splits the right pane horizontally.
The right panes `cd` into directories configured in `~/.wssh.yaml` under `settings.init_layout.top_dir` and `settings.init_layout.bottom_dir`.
* **Auth Check:**
```sh
wssh auth

```


Checks SSH key expiration and primes SSH agents as configured.
* **Macros:**
```sh
wssh macro <macro-name>

```


Injects a macro (predefined command) into the active iTerm2 session.
**Note:** This command is specifically designed to be bound to an iTerm2 keyboard shortcut so you can execute macros while inside a remote SSH session.
**To set this up in iTerm2:**
1. Go to **Preferences** (`Cmd + ,`) > **Keys** > **Key Bindings**.
2. Click the **+** to add a new shortcut (e.g., `Cmd + Shift + L`).
3. Set the Action to **Run Coprocess**.
4. Set the Parameter to: `/usr/local/bin/wssh macro <macro-name>`



### Shell Completion

Configured host aliases, layouts, macros, commands, and flags support tab completion. Enable it in your shell first.

For zsh, add the following to `~/.zshrc`, after any existing `compinit` setup:

```sh
autoload -Uz compinit
compinit
source <(wssh completion zsh)
```

If your shell already runs `compinit`, only the `source` line is needed. Restart your shell, then type `wssh prod` and press Tab to complete matching configured aliases. Layout completion also works after `wssh last` and `wssh --last`.

For bash (with bash-completion installed), add `source <(wssh completion bash)` to `~/.bashrc`. Fish and PowerShell scripts are available through `wssh completion fish` and `wssh completion powershell`.

### Example Layouts

* `single`: Standard pane
* `2h`: Two horizontal panes
* `2v`: Two vertical panes
* `3h`: Three horizontal panes
* `3v`: Three vertical panes
* `4g`: 2x2 grid

## Configuration Example

Here is a basic example of the `~/.wssh.yaml` configuration structure:

```yaml
settings:
  agent_expiration_hours: 23.5
  ignore_key_changes: true
macros:
  check_logs: "tail -f /var/log/syslog"
  restart_app: "sudo systemctl restart myapp"
groups:
  - name: "Production"
    log_session: true
    hosts:
      - alias: "prod-db-01"
      - alias: "prod-web-01"

```

## Notes

* SSH keys and agent configuration are managed via `~/.wssh.yaml`.
* Session logs are saved in `~/wssh_logs` if enabled.
* The application is designed for use with iTerm2 on macOS only.

## License

See LICENSE file for details.
