# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release with EC2 SSM connection support
- RDS IAM authentication token generation
- Interactive mode selection
- AWS profile and region support
- Multi-platform binary builds via GitHub Actions
- Automatic version bumping and releases

### Features
- List and connect to EC2 instances via AWS SSM Session Manager
- List RDS instances and generate IAM auth tokens
- Support for MySQL, MariaDB, and PostgreSQL databases
- Username validation for RDS connections
- Filtering of Oracle databases (no IAM auth support)

### Security
- Release workflow: PR label names are passed to the version-bump script through the environment instead of being pasted into it
- The RDS token is printed once and no longer repeated inside the example commands; the examples read it from `$RDS_TOKEN`
- `mysql`/`psql` examples now verify the server certificate against the RDS CA bundle (`VERIFY_IDENTITY` / `verify-full`)
- **Behaviour change:** `config.yaml` is no longer read from the current directory; use `~/.aws-go-tools/config.yaml`, `/etc/aws-go-tools/config.yaml` or the new `--config` flag
- Shell values from the config file are validated (bare executable path, no spaces or arguments); an invalid or malformed config file is now an error
- Build with Go 1.27.4 (Go 1.25.0 had reachable standard-library vulnerabilities) and run `govulncheck` on every PR
