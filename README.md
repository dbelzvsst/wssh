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
