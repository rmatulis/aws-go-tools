# AWS Go Tools

A command-line tool to manage AWS resources including EC2 instances via Systems Manager (SSM) Session Manager and RDS IAM authentication tokens.

## Features

- 📋 List all EC2 instances in your AWS account
- 🔍 Filter and display instance details (ID, Name, State, Type, IPs)
- 🔗 Interactive instance selection with a user-friendly prompt
- 🚀 Connect to EC2 instances via AWS SSM Session Manager
- 🗄️ List RDS database instances
- 🔑 Generate IAM authentication tokens for RDS databases
- 🌐 Lookup Elastic Network Interfaces (ENI) by IP address and identify associated AWS services
- 🔐 Support for AWS profiles and regions
- ⚡ Built with Cobra CLI framework and Survey for interactive prompts
- ✅ Comprehensive test coverage

## Prerequisites

### For EC2 SSM Connections

1. **AWS CLI configured** with appropriate credentials
2. **Session Manager Plugin** installed - [Installation Guide](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
3. **IAM Permissions** required:
   - `ec2:DescribeInstances`
   - `ssm:StartSession`
   - `ssm:TerminateSession`
4. **EC2 Instance Requirements**:
   - Instance must have SSM Agent installed and running
   - Instance must have an IAM role with `AmazonSSMManagedInstanceCore` policy attached
   - Instance must be in "running" state

### For RDS IAM Auth Tokens

1. **AWS CLI configured** with appropriate credentials
2. **IAM Permissions** required:
   - `rds:DescribeDBInstances`
   - `rds-db:connect` (for the specific database resource)
3. **RDS Database Requirements**:
   - IAM database authentication must be enabled on the RDS instance
   - Database user must be configured for IAM authentication
   - SSL/TLS connection capability

### For ENI Lookup

1. **AWS CLI configured** with appropriate credentials
2. **IAM Permissions** required:
   - `ec2:DescribeNetworkInterfaces`
   - `ec2:DescribeInstances` (optional, for enhanced EC2 instance details)

## Installation

### Option 1: Download pre-built binary (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/rmatulis/aws-go-tools/releases):

```bash
# Linux AMD64
wget https://github.com/rmatulis/aws-go-tools/releases/latest/download/aws-go-tools-linux-amd64
chmod +x aws-go-tools-linux-amd64
sudo mv aws-go-tools-linux-amd64 /usr/local/bin/aws-go-tools

# macOS ARM64 (Apple Silicon)
wget https://github.com/rmatulis/aws-go-tools/releases/latest/download/aws-go-tools-darwin-arm64
chmod +x aws-go-tools-darwin-arm64
sudo mv aws-go-tools-darwin-arm64 /usr/local/bin/aws-go-tools

# macOS AMD64 (Intel)
wget https://github.com/rmatulis/aws-go-tools/releases/latest/download/aws-go-tools-darwin-amd64
chmod +x aws-go-tools-darwin-amd64
sudo mv aws-go-tools-darwin-amd64 /usr/local/bin/aws-go-tools

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/rmatulis/aws-go-tools/releases/latest/download/aws-go-tools-windows-amd64.exe" -OutFile "aws-go-tools.exe"
```

### Option 2: Build from source

```bash
# Clone the repository
git clone https://github.com/rmatulis/aws-go-tools.git
cd aws-go-tools

# Download dependencies
go mod download

# Build the binary (using Makefile - recommended)
make build

# Or build manually
go build -o aws-go-tools

# (Optional) Install to your PATH
make install
# Or manually
go install
```

### Option 3: Run directly

```bash
go run main.go ec2 --profile your-profile
```

### Check Version

```bash
./aws-go-tools version
```

## Development

### Makefile Commands

The project includes a Makefile for common development tasks:

```bash
make help        # Show all available commands
make build       # Build the binary
make build-all   # Build for all platforms
make install     # Install to /usr/local/bin
make clean       # Remove build artifacts
make test        # Run tests
make fmt         # Format code
make version     # Show binary version
```

Build with custom version:
```bash
make build VERSION=v1.2.3
```

### Running Tests

```bash
# Run all tests
go test -v ./...

# Or use the Makefile
make test

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Usage

### View Available Commands

```bash
# Show all available commands
./aws-go-tools --help

# Show help for a specific command
./aws-go-tools ec2 --help
./aws-go-tools rds --help
```

### EC2 SSM Connection Mode

```bash
# Connect to EC2 instance via SSM
./aws-go-tools ec2

# With profile and region
./aws-go-tools ec2 --profile production --region us-west-2

# Or use short flags
./aws-go-tools ec2 -p production -r us-west-2
```

### RDS IAM Auth Token Mode

```bash
# Generate RDS authentication token
./aws-go-tools rds

# With profile and region
./aws-go-tools rds --profile production --region us-east-1

# Or use short flags
./aws-go-tools rds -p production -r us-east-1
```

### ENI Lookup Mode

```bash
# Lookup ENI by private IP address
./aws-go-tools eni 10.0.1.100

# Lookup ENI by public IP address
./aws-go-tools eni 54.123.45.67

# With profile and region
./aws-go-tools eni 10.0.1.100 --profile production --region us-west-2

# Or use short flags
./aws-go-tools eni 10.0.1.100 -p production -r us-west-2
```

### Command Line Options

| Flag | Short | Description | Required | Default |
|------|-------|-------------|----------|---------|  
| `--profile` | `-p` | AWS profile name from ~/.aws/credentials | No | Default profile |
| `--region` | `-r` | AWS region | No | Default region from profile |
| `--config` | `-c` | Path to a config file (see [CONFIG.md](CONFIG.md)) | No | `~/.aws-go-tools/config.yaml`, then `/etc/aws-go-tools/config.yaml` (Linux and macOS) |

### Available Commands

| Command | Description |
|---------|-------------|
| `ec2` | Connect to EC2 instance via SSM |
| `rds` | Generate RDS IAM authentication token |
| `eni` | Lookup ENI by IP address and identify AWS service |
| `version` | Print version information |
| `help` | Help about any command |

### EC2 SSM Connection Flow

1. **List Instances**: The tool queries EC2 to get all instances (excluding terminated ones)
2. **Display Table**: Shows a formatted table with instance details
3. **Interactive Selection**: Uses an interactive prompt to select an instance
4. **SSM Connection**: Establishes an SSM session using the local session-manager-plugin

### RDS IAM Auth Token Flow

1. **List RDS Instances**: The tool queries RDS to get all database instances
2. **Display Table**: Shows a formatted table with RDS instance details
3. **Interactive Selection**: Uses an interactive prompt to select a database
4. **Username Input**: Prompts for the database username
5. **Token Generation**: Generates a temporary IAM authentication token (valid for 15 minutes)
6. **Display Token & Examples**: Shows the token and connection examples for MySQL/PostgreSQL

## Example Output

### Command Help

```
$ ./aws-go-tools --help
A CLI tool to connect to EC2 instances via SSM and generate RDS IAM authentication tokens.

Usage:
  aws-go-tools [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  ec2         Connect to EC2 instance via SSM
  help        Help about any command
  rds         Generate RDS IAM authentication token
  version     Print version information

Flags:
  -h, --help             help for aws-go-tools
  -p, --profile string   AWS profile to use
  -r, --region string    AWS region (optional, uses default region if not specified)
```

### EC2 SSM Connection

```
Available EC2 Instances:
========================================================================================================================
INDEX   INSTANCE ID          NAME                            STATE       TYPE          PRIVATE IP       PUBLIC IP
-----   -------------------  ------------------------------  ----------  ------------  ---------------  ---------------
1       i-0123456789abcdef0  web-server-prod                running     t3.medium     10.0.1.100       54.123.45.67
2       i-0abcdef123456789   app-server-staging             running     t3.large      10.0.2.50        -
3       i-0fedcba987654321   database-server                stopped     t3.xlarge     10.0.3.25        -
========================================================================================================================

? Select an EC2 instance to connect:
▸ web-server-prod (i-0123456789abcdef0) - running
  app-server-staging (i-0abcdef123456789) - running
  database-server (i-0fedcba987654321) - stopped

Starting SSM session to web-server-prod (i-0123456789abcdef0)...
Connected! Type 'exit' to close the session.

sh-4.2$ 
```

### RDS IAM Auth Token Generation

```
Available RDS Instances:
========================================================================================================================
INDEX   IDENTIFIER                                ENGINE           STATUS           ENDPOINT                                            PORT
-----   ----------------------------------------  ---------------  ---------------  --------------------------------------------------  ------
1       production-mysql-db                       mysql            available        prod-db.abc123xyz.us-east-1.rds.amazonaws.com       3306
2       staging-postgres-db                       postgres         available        staging-db.xyz789abc.us-east-1.rds.amazonaws.com    5432
========================================================================================================================

? Select an RDS instance:
▸ production-mysql-db (mysql) - available
  staging-postgres-db (postgres) - available

Enter database username: admin

Generating IAM authentication token for:
  Instance: production-mysql-db
  Endpoint: prod-db.abc123xyz.us-east-1.rds.amazonaws.com:3306
  Username: admin
  Region:   us-east-1

IAM Authentication Token (valid for 15 minutes):
========================================================================================================================
prod-db.abc123xyz.us-east-1.rds.amazonaws.com:3306/?Action=connect&DBUser=admin&X-Amz-Algorithm=...
========================================================================================================================

Load the token into your shell without putting it in history:
  read -rs RDS_TOKEN && export RDS_TOKEN     # paste the token, press Enter
Download the RDS CA bundle once:
  curl -fsSLo global-bundle.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem

Connection Examples:

MySQL/MariaDB:
  MYSQL_PWD="$RDS_TOKEN" mysql -h prod-db.abc123xyz.us-east-1.rds.amazonaws.com -P 3306 -u 'admin' --enable-cleartext-plugin --ssl-mode=VERIFY_IDENTITY --ssl-ca=./global-bundle.pem

Notes:
  - Token is valid for 15 minutes from generation time
  - IAM database authentication must be enabled on the RDS instance
  - The database user must be configured to use IAM authentication
  - SSL/TLS with certificate verification is required; the examples verify against the RDS CA bundle
  - Generated at: 2025-12-23T10:30:00Z
```

### ENI Lookup

```
$ ./aws-go-tools eni 10.0.1.100

Looking up ENI for IP address: 10.0.1.100

ENI Information:
====================================================================================================
ENI ID:         eni-0123456789abcdef0
Status:         in-use
Private IP:     10.0.1.100
Public IP:      54.123.45.67
VPC ID:         vpc-0a1b2c3d4e5f6g7h8 (Production VPC)
Subnet ID:      subnet-0123456789abcdef (Public Subnet 1A)
Service Type:   EC2 Instance
Attached To:    i-0123456789abcdef0 (web-server-prod)
Description:    Primary network interface
====================================================================================================
```

**ENI Lookup with Lambda Function:**
```
$ ./aws-go-tools eni 10.0.2.50

Looking up ENI for IP address: 10.0.2.50

ENI Information:
====================================================================================================
ENI ID:         eni-0fedcba987654321a
Status:         in-use
Private IP:     10.0.2.50
Public IP:      -
VPC ID:         vpc-0a1b2c3d4e5f6g7h8 (Production VPC)
Subnet ID:      subnet-0fedcba987654321 (Private Subnet 1A)
Service Type:   Lambda Function
Attached To:    AWS Lambda VPC ENI
Description:    AWS Lambda VPC ENI-my-lambda-function-abc123
====================================================================================================
```

**ENI Lookup with RDS Database:**
```
$ ./aws-go-tools eni 10.0.3.25

Looking up ENI for IP address: 10.0.3.25

ENI Information:
====================================================================================================
ENI ID:         eni-0abc123def456789b
Status:         in-use
Private IP:     10.0.3.25
Public IP:      -
VPC ID:         vpc-0a1b2c3d4e5f6g7h8
Subnet ID:      subnet-0abc123def456789
Service Type:   RDS Database Instance
Attached To:    RDSNetworkInterface
Description:    RDSNetworkInterface
====================================================================================================
```

## Troubleshooting

### EC2 SSM Issues

#### "session-manager-plugin not found"
Install the Session Manager plugin from [AWS Documentation](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

#### "failed to start session"
- Verify the instance has SSM Agent installed and running
- Check that the instance has the required IAM role with SSM permissions
- Ensure the instance is in "running" state
- Verify your IAM user has `ssm:StartSession` permission

#### "Cannot interrupt commands with Ctrl+C in SSM session"

**This issue has been resolved!** The tool now automatically puts your terminal in raw mode during SSM sessions, which allows control characters (Ctrl+C, Ctrl+Z, etc.) to pass through correctly to the remote shell.

**What was fixed:**
- Terminal is now set to raw mode before starting the SSM session
- Control characters are passed through directly to the remote shell as raw bytes
- SIGINT is ignored in the parent process so Ctrl+C only affects the remote command, not the session
- Terminal state is automatically restored when the session ends
- Escape sequences like `^[[27;5;99~` no longer appear

**How to use:**
Simply connect normally and Ctrl+C will now interrupt the remote command (not the session):
```bash
./aws-go-tools ec2
# Once connected, run:
tail -f /var/log/syslog
# Press Ctrl+C to interrupt the tail command
# The session remains active - you can run more commands!
# Type 'exit' when you want to close the session
```

**Important:** Ctrl+C now works correctly to stop remote processes without disconnecting the session. To end the session, type `exit`.

**If you still experience issues (rare):**

Some terminal emulators or SSH forwarding scenarios might still have limitations. Here are workarounds:

**Option 1: Use alternative commands**
```bash
# Instead of tail -f, use tail with a line limit
tail -n 100 /var/log/syslog

# Or use timeout to automatically exit
timeout 30 tail -f /var/log/syslog

# Use less instead of tail for log viewing
less +F /var/log/syslog  # Press Ctrl+C to stop following, then 'q' to quit
```

**Option 2: Use background processes**
```bash
# Run the command in background and kill it
tail -f /var/log/syslog &
# Note the PID, then kill it when done
kill <PID>
```

**Option 3: Use screen or tmux**
```bash
# Start a screen session
screen
# Run your command
tail -f /var/log/syslog
# Detach with Ctrl+A then D
# Reattach later with: screen -r
```

#### "No EC2 instances found"
- Verify you're using the correct AWS profile and region
- Check your IAM permissions include `ec2:DescribeInstances`
- Ensure you have EC2 instances in the specified region

### RDS IAM Auth Issues

#### "Failed to generate auth token"
- Verify your AWS credentials have permission for `rds-db:connect`
- Check that you have network connectivity to RDS
- Ensure the region is correctly specified

#### "Connection refused" when using token
- Verify IAM database authentication is enabled on the RDS instance
- Check that the database user is created and configured for IAM authentication

### ENI Lookup Issues

#### "No ENI found with IP address"
- Verify the IP address is correct and belongs to an ENI in the current region
- Check you're using the correct AWS profile and region
- Ensure you have ENIs in the specified region
- The IP address could be in a different AWS region

#### "Failed to find ENI"
- Verify your IAM user has `ec2:DescribeNetworkInterfaces` permission
- Check your AWS credentials are properly configured
- Ensure network connectivity to AWS API endpoints
  - MySQL: `CREATE USER admin IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';`
  - PostgreSQL: `CREATE USER admin WITH LOGIN; GRANT rds_iam TO admin;`
- Ensure you're using SSL/TLS for the connection
- Verify security groups allow your IP address
- Check that the token hasn't expired (15-minute validity)

#### "No RDS instances found"
- Verify you're using the correct AWS profile and region
- Check your IAM permissions include `rds:DescribeDBInstances`
- Ensure you have RDS instances in the specified region

## Release Management

### Automated Releases

This project uses GitHub Actions for automated building and releasing. When a pull request is merged to the `main` branch:

1. **Version Bumping**: Automatically increments version based on PR labels (defaults to patch)
2. **Multi-platform Builds**: Builds binaries for Linux, macOS, and Windows (AMD64 and ARM64)
3. **GitHub Release**: Creates a new release with all binaries attached

### Version Bumping with PR Labels

Version bumps are determined by labels on the pull request:

- `major` or `breaking` label → Major version bump (1.0.0 → 2.0.0)
- `minor` or `feature` label → Minor version bump (1.0.0 → 1.1.0)
- No label or `patch` label → Patch version bump (1.0.0 → 1.0.1)

**Workflow:**
1. Create a pull request with your changes
2. Add appropriate label to the PR:
   - `major` - Breaking changes
   - `minor` or `feature` - New features
   - `patch` or no label - Bug fixes (default)
3. Merge the PR to `main`
4. GitHub Actions automatically builds and releases

### Continuous Integration

Every pull request automatically runs:
- **Linting & Formatting**: Code style and formatting checks (`go fmt`, `go vet`, `golangci-lint`)
- **Tests**: All unit tests with race detection and coverage reports
- **Build Validation**: Ensures code builds successfully for all supported platforms (Linux, macOS, Windows - AMD64 & ARM64)

Check results are posted as a comment on your PR, making it easy to see the status at a glance.

### Manual Release

To trigger a release manually:
1. Go to Actions tab in GitHub
2. Select "Build and Release" workflow
3. Click "Run workflow"

## Security Notes

### EC2 SSM

- This tool uses AWS SSM Session Manager, which provides secure shell access without opening inbound ports
- All session activity can be logged to CloudWatch Logs or S3 (configure in Session Manager preferences)
- No SSH keys are required or used
- Sessions are encrypted using TLS 1.2+

### RDS IAM Authentication

- IAM authentication tokens are temporary (15-minute validity)
- Tokens are generated locally using your AWS credentials
- No database passwords are stored or transmitted
- SSL/TLS is required for all IAM-authenticated database connections
- Authentication is based on AWS IAM identity, providing centralized access control
- All database connections using IAM auth are logged in CloudTrail

## Dependencies

- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- [promptui](https://github.com/manifoldco/promptui) - Interactive CLI prompts

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
