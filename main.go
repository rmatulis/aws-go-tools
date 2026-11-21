package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Version information (set by build flags)
var Version = "dev"

// Global flags
var (
	profile    string
	region     string
	configFile string
)

type Instance struct {
	ID           string
	Name         string
	PrivateIP    string
	PublicIP     string
	State        string
	InstanceType string
	Platform     string
}

type RDSInstance struct {
	Identifier string
	Endpoint   string
	Port       int32
	Engine     string
	Status     string
}

// ENIInfo represents information about an ENI and its attachments
type ENIInfo struct {
	ENIID          string
	PrivateIP      string
	PublicIP       string
	Status         string
	SubnetID       string
	SubnetName     string
	VPCID          string
	VPCName        string
	AttachmentType string
	AttachedTo     string
	Description    string
}

// SessionData represents the data structure for SSM session manager plugin
type SessionData struct {
	SessionId  string `json:"SessionId"`
	StreamUrl  string `json:"StreamUrl"`
	TokenValue string `json:"TokenValue"`
}

// Config represents the configuration file structure
type Config struct {
	Linux   ShellConfig `yaml:"linux"`
	Windows ShellConfig `yaml:"windows"`
}

// ShellConfig represents shell configuration for a platform
type ShellConfig struct {
	Shell string `yaml:"shell"`
}

// Global configuration
var appConfig Config

func main() {
	rootCmd := &cobra.Command{
		Use:   "aws-go-tools",
		Short: "AWS tools for EC2 SSM connections and RDS IAM authentication",
		Long:  `A CLI tool to connect to EC2 instances via SSM and generate RDS IAM authentication tokens.`,
		// main prints the error itself; without this Cobra prints it a second time
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configFile, defaultConfigPaths())
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			appConfig = cfg
			return nil
		},
	}

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aws-go-tools version %s\n", Version)
		},
	}

	// EC2 command
	ec2Cmd := &cobra.Command{
		Use:   "ec2",
		Short: "Connect to EC2 instance via SSM",
		Long:  `List EC2 instances and connect to the selected instance using AWS Systems Manager Session Manager.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			cfg := loadAWSConfig(ctx)
			handleEC2Mode(ctx, cfg)
		},
	}

	// RDS command
	rdsCmd := &cobra.Command{
		Use:   "rds",
		Short: "Generate RDS IAM authentication token",
		Long:  `List RDS instances and generate an IAM authentication token for the selected instance.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			cfg := loadAWSConfig(ctx)
			handleRDSMode(ctx, cfg)
		},
	}

	// ENI lookup command
	eniCmd := &cobra.Command{
		Use:   "eni [ip-address]",
		Short: "Lookup ENI by IP address and identify associated AWS service",
		Long:  `Find the network interface associated with an IP address and determine which AWS service it belongs to.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			cfg := loadAWSConfig(ctx)
			handleENILookup(ctx, cfg, args[0])
		},
	}

	// Add persistent flags
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile to use")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region (optional, uses default region if not specified)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to a config file (default: ~/.aws-go-tools/config.yaml, then /etc/aws-go-tools/config.yaml)")

	// Add commands
	rootCmd.AddCommand(versionCmd, ec2Cmd, rdsCmd, eniCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Shell values are sent to the instance as the SSM session command, so they
// must be a bare executable path: no spaces, arguments or shell metacharacters.
var shellPatterns = map[string]*regexp.Regexp{
	"linux":   regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`),
	"windows": regexp.MustCompile(`^([A-Za-z]:\\)?[A-Za-z0-9._\\-]+\.exe$`),
}

func validateShell(platform, shell string) error {
	pattern, ok := shellPatterns[platform]
	if !ok {
		return fmt.Errorf("unknown platform %q", platform)
	}
	if !pattern.MatchString(shell) {
		return fmt.Errorf("invalid %s shell %q: must be a bare executable path with no spaces or arguments", platform, shell)
	}
	return nil
}

// defaultConfigPaths returns the config locations searched when --config is
// not given. Every path is absolute: the working directory is never searched,
// so a config.yaml planted in a cloned repository or shared directory cannot
// choose the command that runs on a remote instance.
func defaultConfigPaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		paths = append(paths, filepath.Join(home, ".aws-go-tools", "config.yaml"))
	}
	return append(paths, "/etc/aws-go-tools/config.yaml")
}

// loadConfig returns the defaults overlaid with explicitPath if set, otherwise
// with the first of searchPaths that exists. A file that exists but cannot be
// read, parsed or validated is an error rather than a fallback.
func loadConfig(explicitPath string, searchPaths []string) (Config, error) {
	cfg := Config{
		Linux:   ShellConfig{Shell: "/bin/bash"},
		Windows: ShellConfig{Shell: "powershell.exe"},
	}

	paths := searchPaths
	if explicitPath != "" {
		paths = []string{explicitPath}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 G703 -- path is the --config flag or a fixed per-user/system location
		if err != nil {
			if explicitPath == "" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return Config{}, fmt.Errorf("failed to read config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse config %s: %w", path, err)
		}
		if err := validateShell("linux", cfg.Linux.Shell); err != nil {
			return Config{}, fmt.Errorf("config %s: %w", path, err)
		}
		if err := validateShell("windows", cfg.Windows.Shell); err != nil {
			return Config{}, fmt.Errorf("config %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "Loaded configuration from: %s\n", path)
		return cfg, nil
	}

	return cfg, nil
}

func loadAWSConfig(ctx context.Context) aws.Config {
	configOptions := []func(*config.LoadOptions) error{}

	if profile != "" {
		configOptions = append(configOptions, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		configOptions = append(configOptions, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	return cfg
}

func handleEC2Mode(ctx context.Context, cfg aws.Config) {
	// List EC2 instances
	instances, err := listInstances(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	if len(instances) == 0 {
		fmt.Println("No EC2 instances found")
		return
	}

	// Prompt user to select an instance
	selectedInstance, err := selectInstance(instances)
	if err != nil {
		log.Fatalf("Failed to select instance: %v", err)
	}

	// Connect via SSM
	err = connectToInstance(ctx, cfg, selectedInstance)
	if err != nil {
		log.Fatalf("Failed to connect to instance: %v", err)
	}
}

func handleRDSMode(ctx context.Context, cfg aws.Config) {
	// List RDS instances
	rdsInstances, err := listRDSInstances(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to list RDS instances: %v", err)
	}

	if len(rdsInstances) == 0 {
		fmt.Println("No RDS instances found")
		return
	}

	// Prompt user to select an RDS instance
	selectedRDS, err := selectRDSInstance(rdsInstances)
	if err != nil {
		log.Fatalf("Failed to select RDS instance: %v", err)
	}

	// Prompt for username
	username, err := promptForUsername()
	if err != nil {
		log.Fatalf("Failed to get username: %v", err)
	}

	// Generate IAM auth token
	err = generateRDSAuthToken(ctx, cfg, selectedRDS, username)
	if err != nil {
		log.Fatalf("Failed to generate auth token: %v", err)
	}
}

func listInstances(ctx context.Context, cfg aws.Config) ([]Instance, error) {
	ec2Client := ec2.NewFromConfig(cfg)

	// Describe all instances
	result, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []Instance

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			// Skip terminated instances
			if instance.State.Name == types.InstanceStateNameTerminated {
				continue
			}

			inst := Instance{
				ID:           aws.ToString(instance.InstanceId),
				InstanceType: string(instance.InstanceType),
				State:        string(instance.State.Name),
			}

			// Determine platform (defaults to Linux if not specified)
			inst.Platform = "linux"
			if instance.Platform != "" {
				platform := strings.ToLower(string(instance.Platform))
				if platform == "windows" {
					inst.Platform = "windows"
				}
			}
			// Also check PlatformDetails for more accurate detection
			if instance.PlatformDetails != nil {
				platformDetails := strings.ToLower(aws.ToString(instance.PlatformDetails))
				if strings.Contains(platformDetails, "windows") {
					inst.Platform = "windows"
				}
			}

			// Get instance name from tags
			for _, tag := range instance.Tags {
				if aws.ToString(tag.Key) == "Name" {
					inst.Name = aws.ToString(tag.Value)
					break
				}
			}

			// Get IP addresses
			if instance.PrivateIpAddress != nil {
				inst.PrivateIP = aws.ToString(instance.PrivateIpAddress)
			}
			if instance.PublicIpAddress != nil {
				inst.PublicIP = aws.ToString(instance.PublicIpAddress)
			}

			instances = append(instances, inst)
		}
	}

	return instances, nil
}

func selectInstance(instances []Instance) (Instance, error) {
	var options []string
	for _, inst := range instances {
		options = append(options, fmt.Sprintf("%s (%s) - %s", inst.Name, inst.ID, inst.State))
	}

	var selected string
	prompt := &survey.Select{
		Message:  "Select an EC2 instance to connect:",
		Options:  options,
		PageSize: 10,
	}

	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return Instance{}, err
	}

	// Find the selected instance by matching the formatted string
	for i, opt := range options {
		if opt == selected {
			return instances[i], nil
		}
	}

	return Instance{}, fmt.Errorf("selected instance not found")
}

func connectToInstance(ctx context.Context, cfg aws.Config, instance Instance) error {
	// Check if instance is running
	if instance.State != string(types.InstanceStateNameRunning) {
		return fmt.Errorf("instance %s is not in running state (current state: %s)", instance.ID, instance.State)
	}

	// Check if session-manager-plugin is installed
	if _, err := exec.LookPath("session-manager-plugin"); err != nil {
		return fmt.Errorf("session-manager-plugin not found. Please install it from: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}

	ssmClient := ssm.NewFromConfig(cfg)

	// Determine which shell to use based on platform
	var shellCommand string
	if instance.Platform == "windows" {
		shellCommand = appConfig.Windows.Shell
	} else {
		shellCommand = appConfig.Linux.Shell
	}

	// Start SSM session
	fmt.Printf("\nStarting SSM session to %s (%s) with %s shell...\n", instance.Name, instance.ID, shellCommand)

	startSessionInput := &ssm.StartSessionInput{
		Target:       aws.String(instance.ID),
		DocumentName: aws.String("AWS-StartInteractiveCommand"),
		Parameters: map[string][]string{
			"command": {shellCommand},
		},
	}

	result, err := ssmClient.StartSession(ctx, startSessionInput)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Build the session-manager-plugin command
	sessionID := aws.ToString(result.SessionId)
	streamURL := aws.ToString(result.StreamUrl)
	tokenValue := aws.ToString(result.TokenValue)

	// Get the region from config
	region := cfg.Region

	// Prepare the session data JSON using proper marshaling
	sessionDataStruct := SessionData{
		SessionId:  sessionID,
		StreamUrl:  streamURL,
		TokenValue: tokenValue,
	}

	sessionDataBytes, err := json.Marshal(sessionDataStruct)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	sessionData := string(sessionDataBytes)

	// Execute session-manager-plugin
	cmd := exec.Command(
		"session-manager-plugin",
		sessionData,
		region,
		"StartSession",
	)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Check if stdin is a terminal and set up raw mode for proper control character handling
	// This ensures Ctrl+C and other control sequences are passed through correctly to the remote shell
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		// Save the current terminal state
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			log.Printf("Warning: Failed to set terminal to raw mode: %v", err)
		} else {
			// Ensure we restore the terminal state when done
			defer func() {
				if err := term.Restore(fd, oldState); err != nil {
					log.Printf("Warning: Failed to restore terminal state: %v", err)
				}
			}()
		}
	}

	// Ignore SIGINT in the parent process so Ctrl+C doesn't terminate the session
	// In raw mode, Ctrl+C sends literal bytes (0x03) through stdin to the remote shell
	// We don't want the signal to also terminate our local session-manager-plugin process
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt) // Restore default signal handling when done

	fmt.Println("Connected! Type 'exit' to close the session.")
	fmt.Println("Ctrl+C will interrupt commands running in the session.")

	// Run the session-manager-plugin
	// In raw mode, all control characters pass through to the remote shell as raw bytes
	if err := cmd.Run(); err != nil {
		fmt.Println("\nSession ended.")
		return fmt.Errorf("session-manager-plugin error: %w", err)
	}

	fmt.Println("\nSession ended.")
	return nil
}

// RDS-related functions

func listRDSInstances(ctx context.Context, cfg aws.Config) ([]RDSInstance, error) {
	rdsClient := rds.NewFromConfig(cfg)

	// Describe all RDS instances
	result, err := rdsClient.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe RDS instances: %w", err)
	}

	var instances []RDSInstance

	for _, dbInstance := range result.DBInstances {
		engine := aws.ToString(dbInstance.Engine)

		// Skip Oracle databases as they don't support IAM authentication
		if strings.Contains(strings.ToLower(engine), "oracle") {
			continue
		}

		inst := RDSInstance{
			Identifier: aws.ToString(dbInstance.DBInstanceIdentifier),
			Engine:     engine,
			Status:     aws.ToString(dbInstance.DBInstanceStatus),
		}

		if dbInstance.Endpoint != nil {
			inst.Endpoint = aws.ToString(dbInstance.Endpoint.Address)
			if dbInstance.Endpoint.Port != nil {
				inst.Port = *dbInstance.Endpoint.Port
			}
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

func selectRDSInstance(instances []RDSInstance) (RDSInstance, error) {
	var options []string
	for _, inst := range instances {
		options = append(options, fmt.Sprintf("%s (%s) - %s", inst.Identifier, inst.Engine, inst.Status))
	}

	var selected string
	prompt := &survey.Select{
		Message:  "Select an RDS instance:",
		Options:  options,
		PageSize: 10,
	}

	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return RDSInstance{}, err
	}

	// Find the selected instance by matching the formatted string
	for i, opt := range options {
		if opt == selected {
			return instances[i], nil
		}
	}

	return RDSInstance{}, fmt.Errorf("selected RDS instance not found")
}

func promptForUsername() (string, error) {
	var username string
	prompt := &survey.Input{
		Message: "Enter database username:",
	}

	err := survey.AskOne(prompt, &username, survey.WithValidator(survey.Required))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(username), nil
}

func generateRDSAuthToken(ctx context.Context, cfg aws.Config, instance RDSInstance, username string) error {
	if instance.Status != "available" {
		fmt.Printf("Warning: RDS instance %s is not in 'available' state (current state: %s)\n", instance.Identifier, instance.Status)
	}

	if instance.Endpoint == "" {
		return fmt.Errorf("RDS instance %s does not have an endpoint", instance.Identifier)
	}

	// Build the endpoint address with port
	endpoint := fmt.Sprintf("%s:%d", instance.Endpoint, instance.Port)

	fmt.Printf("\nGenerating IAM authentication token for:\n")
	fmt.Printf("  Instance: %s\n", instance.Identifier)
	fmt.Printf("  Endpoint: %s\n", endpoint)
	fmt.Printf("  Username: %s\n", username)
	fmt.Printf("  Region:   %s\n", cfg.Region)
	fmt.Println()

	// Generate the auth token (valid for 15 minutes)
	authToken, err := auth.BuildAuthToken(ctx, endpoint, cfg.Region, username, cfg.Credentials)
	if err != nil {
		return fmt.Errorf("failed to generate auth token: %w", err)
	}

	fmt.Println("IAM Authentication Token (valid for 15 minutes):")
	fmt.Println(strings.Repeat("=", 120))
	fmt.Println(authToken)
	fmt.Println(strings.Repeat("=", 120))
	fmt.Println()

	// Display connection examples
	fmt.Println("Connection Examples:")
	fmt.Println()

	// MySQL/MariaDB
	if strings.Contains(strings.ToLower(instance.Engine), "mysql") || strings.Contains(strings.ToLower(instance.Engine), "mariadb") {
		fmt.Println("MySQL/MariaDB:")
		fmt.Printf("  mysql -h %s -P %d -u %s --password='%s' --enable-cleartext-plugin --ssl-mode=REQUIRED\n",
			instance.Endpoint, instance.Port, username, authToken)
		fmt.Println()
	}

	// PostgreSQL
	if strings.Contains(strings.ToLower(instance.Engine), "postgres") {
		fmt.Println("PostgreSQL:")
		fmt.Printf("  psql \"host=%s port=%d dbname=your_database user=%s password=%s sslmode=require\"\n",
			instance.Endpoint, instance.Port, username, authToken)
		fmt.Println()
		fmt.Println("Or using environment variable:")
		fmt.Printf("  export PGPASSWORD='%s'\n", authToken)
		fmt.Printf("  psql -h %s -p %d -U %s -d your_database\n", instance.Endpoint, instance.Port, username)
		fmt.Println()
	}

	fmt.Println("Notes:")
	fmt.Println("  - Token is valid for 15 minutes from generation time")
	fmt.Println("  - IAM database authentication must be enabled on the RDS instance")
	fmt.Println("  - The database user must be configured to use IAM authentication")
	fmt.Println("  - SSL/TLS connection is required")
	fmt.Printf("  - Generated at: %s\n", time.Now().Format(time.RFC3339))

	return nil
}

// ENI Lookup Functions

func handleENILookup(ctx context.Context, cfg aws.Config, ipAddress string) {
	fmt.Printf("\nLooking up ENI for IP address: %s\n\n", ipAddress)

	// Find the ENI
	eniInfo, err := findENIByIP(ctx, cfg, ipAddress)
	if err != nil {
		log.Fatalf("Failed to find ENI: %v", err)
	}

	if eniInfo == nil {
		fmt.Printf("No ENI found with IP address: %s\n", ipAddress)
		return
	}

	// Display ENI information
	displayENIInfo(eniInfo)
}

func findENIByIP(ctx context.Context, cfg aws.Config, ipAddress string) (*ENIInfo, error) {
	ec2Client := ec2.NewFromConfig(cfg)

	// Search for ENI by private IP
	result, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("addresses.private-ip-address"),
				Values: []string{ipAddress},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	// If not found by private IP, try public IP via association
	if len(result.NetworkInterfaces) == 0 {
		result, err = ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("association.public-ip"),
					Values: []string{ipAddress},
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe network interfaces by public IP: %w", err)
		}
	}

	if len(result.NetworkInterfaces) == 0 {
		return nil, nil
	}

	eni := result.NetworkInterfaces[0]

	info := &ENIInfo{
		ENIID:       aws.ToString(eni.NetworkInterfaceId),
		Status:      string(eni.Status),
		SubnetID:    aws.ToString(eni.SubnetId),
		VPCID:       aws.ToString(eni.VpcId),
		Description: aws.ToString(eni.Description),
	}

	// Get VPC name from tags
	if info.VPCID != "" {
		vpcResult, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			VpcIds: []string{info.VPCID},
		})
		if err == nil && len(vpcResult.Vpcs) > 0 {
			for _, tag := range vpcResult.Vpcs[0].Tags {
				if aws.ToString(tag.Key) == "Name" {
					info.VPCName = aws.ToString(tag.Value)
					break
				}
			}
		}
	}

	// Get Subnet name from tags
	if info.SubnetID != "" {
		subnetResult, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			SubnetIds: []string{info.SubnetID},
		})
		if err == nil && len(subnetResult.Subnets) > 0 {
			for _, tag := range subnetResult.Subnets[0].Tags {
				if aws.ToString(tag.Key) == "Name" {
					info.SubnetName = aws.ToString(tag.Value)
					break
				}
			}
		}
	}

	// Get private IP
	if len(eni.PrivateIpAddresses) > 0 {
		info.PrivateIP = aws.ToString(eni.PrivateIpAddresses[0].PrivateIpAddress)
	}

	// Get public IP if associated
	if eni.Association != nil {
		info.PublicIP = aws.ToString(eni.Association.PublicIp)
	}

	// Determine what the ENI is attached to
	if eni.Attachment != nil {
		info.AttachmentType = aws.ToString(eni.Attachment.AttachmentId)
		instanceID := aws.ToString(eni.Attachment.InstanceId)

		if instanceID != "" {
			// Attached to an EC2 instance
			info.AttachedTo = instanceID
			info.AttachmentType = "EC2 Instance"

			// Try to get instance name
			instanceResult, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err == nil && len(instanceResult.Reservations) > 0 && len(instanceResult.Reservations[0].Instances) > 0 {
				instance := instanceResult.Reservations[0].Instances[0]
				for _, tag := range instance.Tags {
					if aws.ToString(tag.Key) == "Name" {
						info.AttachedTo = fmt.Sprintf("%s (%s)", instanceID, aws.ToString(tag.Value))
						break
					}
				}
			}
		}
	}

	// Check if it's attached to other AWS services based on description or requester ID
	if eni.RequesterManaged != nil && *eni.RequesterManaged {
		info.AttachmentType = identifyAWSService(info.Description, aws.ToString(eni.RequesterId))
		info.AttachedTo = info.Description
	}

	// If still no attachment info, check interface type
	if info.AttachmentType == "" {
		switch eni.InterfaceType {
		case types.NetworkInterfaceTypeNatGateway:
			info.AttachmentType = "NAT Gateway"
		case types.NetworkInterfaceTypeNetworkLoadBalancer:
			info.AttachmentType = "Network Load Balancer"
		case types.NetworkInterfaceTypeGatewayLoadBalancer:
			info.AttachmentType = "Gateway Load Balancer"
		case types.NetworkInterfaceTypeGatewayLoadBalancerEndpoint:
			info.AttachmentType = "Gateway Load Balancer Endpoint"
		case types.NetworkInterfaceTypeInterface:
			info.AttachmentType = "Standard Network Interface"
		default:
			info.AttachmentType = string(eni.InterfaceType)
		}
	}

	return info, nil
}

func identifyAWSService(description, requesterID string) string {
	description = strings.ToLower(description)
	requesterID = strings.ToLower(requesterID)

	// Check description for known AWS services
	servicePatterns := map[string]string{
		"rds":                "RDS Database Instance",
		"elb":                "Elastic Load Balancer",
		"elbv2":              "Application/Network Load Balancer",
		"lambda":             "Lambda Function",
		"ecs":                "ECS Task",
		"elasticache":        "ElastiCache Cluster",
		"redshift":           "Redshift Cluster",
		"dax":                "DynamoDB Accelerator (DAX)",
		"aws glue":           "AWS Glue",
		"sagemaker":          "SageMaker",
		"workspaces":         "Amazon WorkSpaces",
		"directory service":  "AWS Directory Service",
		"vpn":                "VPN Connection",
		"transit gateway":    "Transit Gateway",
		"vpc endpoint":       "VPC Endpoint",
		"privatelink":        "PrivateLink",
		"api gateway":        "API Gateway",
		"app runner":         "App Runner",
		"msk":                "Managed Streaming for Kafka (MSK)",
		"documentdb":         "DocumentDB Cluster",
		"neptune":            "Neptune Database",
		"fsx":                "FSx File System",
		"efs mount target":   "EFS Mount Target",
		"transfer":           "AWS Transfer Family",
		"codebuild":          "CodeBuild",
		"cloud9":             "AWS Cloud9",
		"appstream":          "AppStream 2.0",
		"global accelerator": "AWS Global Accelerator",
		"cloudwan":           "AWS Cloud WAN",
		"network firewall":   "AWS Network Firewall",
		"verified access":    "AWS Verified Access",
	}

	for pattern, service := range servicePatterns {
		if strings.Contains(description, pattern) || strings.Contains(requesterID, pattern) {
			return service
		}
	}

	// Check requester ID patterns
	if strings.Contains(requesterID, "amazon") {
		return "AWS Managed Service"
	}

	return "AWS Managed Resource"
}

func displayENIInfo(info *ENIInfo) {
	fmt.Println("ENI Information:")
	fmt.Println(strings.Repeat("=", 100))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	// Write errors are sticky in tabwriter and surface from Flush below
	row := func(label, value string) {
		_, _ = fmt.Fprintf(w, "%s:\t%s\n", label, value)
	}
	withName := func(id, name string) string {
		if name != "" {
			return fmt.Sprintf("%s (%s)", id, name)
		}
		return id
	}

	row("ENI ID", info.ENIID)
	row("Status", info.Status)
	row("Private IP", info.PrivateIP)
	if info.PublicIP != "" {
		row("Public IP", info.PublicIP)
	} else {
		row("Public IP", "-")
	}
	row("VPC ID", withName(info.VPCID, info.VPCName))
	row("Subnet ID", withName(info.SubnetID, info.SubnetName))
	row("Service Type", info.AttachmentType)
	if info.AttachedTo != "" {
		row("Attached To", info.AttachedTo)
	}
	if info.Description != "" {
		row("Description", info.Description)
	}
	if err := w.Flush(); err != nil {
		log.Printf("Warning: Failed to write ENI information: %v", err)
	}
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println()
}
