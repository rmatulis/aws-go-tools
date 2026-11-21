# Configuration Guide

## Overview

AWS Go Tools supports configuration files to customize shell preferences for different server types (Linux and Windows).

## Configuration File Locations

The tool looks for a configuration file in the following order:

1. The path given with `--config <path>` (or `-c <path>`). If this flag is set, only that file is used.
2. `~/.aws-go-tools/config.yaml` in your home directory
3. `/etc/aws-go-tools/config.yaml` (system-wide)

The first configuration file found will be used. If no configuration file is found, default settings are applied.

The current directory is **not** searched. The configured shell is the command that runs on the remote instance, so a `config.yaml` picked up from whatever directory you happen to be in (a cloned repository, a shared folder) could run an arbitrary command on your instances. To use a config file from a project directory, pass it explicitly: `aws-go-tools --config ./config.yaml ec2`.

A configuration file that exists but cannot be read, is not valid YAML, or names an invalid shell makes the tool exit with an error. It does not fall back to the defaults or to the next location.

## Shell Value Rules

A shell must be a bare executable path with no spaces, arguments or shell metacharacters:

- **Linux**: an absolute path made of letters, digits, `.`, `_`, `-` and `/` (for example `/bin/bash`, `/usr/local/bin/fish`)
- **Windows**: an `.exe` name or path made of letters, digits, `.`, `_`, `-` and `\`, optionally starting with a drive letter (for example `powershell.exe`, `C:\Windows\System32\cmd.exe`). Paths containing spaces, such as anything under `C:\Program Files`, are not accepted.

## Default Configuration

If no configuration file is provided, the following defaults are used:

- **Linux instances**: `/bin/bash`
- **Windows instances**: `powershell.exe`

## Configuration File Format

Create a `config.yaml` file with the following structure:

```yaml
# AWS Go Tools Configuration
# Shell configuration for Linux instances
linux:
  shell: /bin/bash

# Shell configuration for Windows instances
windows:
  shell: powershell.exe
```

## Available Shell Options

### Linux Shells

Common shells you can configure for Linux instances:

```yaml
linux:
  shell: /bin/bash      # Bash (default)
  # shell: /bin/sh      # Bourne shell
  # shell: /bin/zsh     # Z shell
  # shell: /bin/fish    # Friendly interactive shell
```

### Windows Shells

Common shells you can configure for Windows instances:

```yaml
windows:
  shell: powershell.exe  # PowerShell (default)
  # shell: cmd.exe       # Command Prompt
```

## Platform Detection

The tool automatically detects the instance platform using:

1. EC2 `Platform` attribute
2. EC2 `PlatformDetails` attribute (more reliable)
3. Defaults to Linux if platform cannot be determined

## Example Configurations

### Using Zsh for Linux

```yaml
linux:
  shell: /bin/zsh
windows:
  shell: powershell.exe
```

### Using Command Prompt for Windows

```yaml
linux:
  shell: /bin/bash
windows:
  shell: cmd.exe
```

### Custom Shell Paths

```yaml
linux:
  shell: /usr/local/bin/fish
windows:
  shell: C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe
```

## Creating a Configuration File

### Per-User Configuration

1. Create the configuration directory:
   ```bash
   mkdir -p ~/.aws-go-tools
   ```

2. Create the configuration file:
   ```bash
   cat > ~/.aws-go-tools/config.yaml << EOF
   linux:
     shell: /bin/bash
   windows:
     shell: powershell.exe
   EOF
   ```

### Project-Specific Configuration

Create a `config.yaml` file in your project directory and pass it explicitly:

```bash
aws-go-tools --config ./config.yaml ec2
```

### System-Wide Configuration

For system-wide settings (requires root/admin):

```bash
sudo mkdir -p /etc/aws-go-tools
sudo nano /etc/aws-go-tools/config.yaml
```

## Troubleshooting

### Configuration Not Loading

If your configuration isn't being applied:

1. Check the file location matches one of the expected paths, or pass it with `--config`
2. Look for the "Loaded configuration from:" message when running the tool
3. If the tool exits with a "failed to parse config" or "invalid ... shell" error, fix the YAML syntax (spaces, not tabs) or the shell value named in the message

### Shell Not Found on Instance

If you configure a shell that doesn't exist on the target instance, the SSM session may fail to start. Ensure:

1. The shell exists on all target instances
2. The shell path is correct for the instance OS
3. The shell is executable

### Windows Shell Issues

For Windows instances:

- Use `powershell.exe` for PowerShell
- Use `cmd.exe` for Command Prompt
- Full paths are typically not required as they're in the system PATH

## Notes

- Configuration is loaded once at startup
- Changes to the configuration file require restarting the tool
- Shell commands must be available on the target instance
- SSM sessions use the `AWS-StartInteractiveCommand` document
