package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestInstanceStruct(t *testing.T) {
	tests := []struct {
		name     string
		instance Instance
		wantID   string
		wantName string
	}{
		{
			name: "Complete instance",
			instance: Instance{
				ID:           "i-123456",
				Name:         "test-instance",
				State:        "running",
				InstanceType: "t3.micro",
				PrivateIP:    "10.0.1.100",
				PublicIP:     "54.123.45.67",
			},
			wantID:   "i-123456",
			wantName: "test-instance",
		},
		{
			name: "Instance without name",
			instance: Instance{
				ID:           "i-789012",
				State:        "stopped",
				InstanceType: "t3.small",
			},
			wantID:   "i-789012",
			wantName: "",
		},
		{
			name: "Instance without public IP",
			instance: Instance{
				ID:           "i-345678",
				Name:         "private-instance",
				State:        "running",
				InstanceType: "t3.medium",
				PrivateIP:    "10.0.2.50",
			},
			wantID:   "i-345678",
			wantName: "private-instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.instance.ID != tt.wantID {
				t.Errorf("Expected ID to be %s, got %s", tt.wantID, tt.instance.ID)
			}
			if tt.instance.Name != tt.wantName {
				t.Errorf("Expected Name to be %s, got %s", tt.wantName, tt.instance.Name)
			}
		})
	}
}

func TestRDSInstanceStruct(t *testing.T) {
	tests := []struct {
		name        string
		rdsInstance RDSInstance
		wantEngine  string
		wantPort    int32
	}{
		{
			name: "PostgreSQL instance",
			rdsInstance: RDSInstance{
				Identifier: "postgres-db",
				Engine:     "postgres",
				Status:     "available",
				Port:       5432,
				Endpoint:   "db.example.com",
			},
			wantEngine: "postgres",
			wantPort:   5432,
		},
		{
			name: "MySQL instance",
			rdsInstance: RDSInstance{
				Identifier: "mysql-db",
				Engine:     "mysql",
				Status:     "available",
				Port:       3306,
				Endpoint:   "mysql.example.com",
			},
			wantEngine: "mysql",
			wantPort:   3306,
		},
		{
			name: "MariaDB instance",
			rdsInstance: RDSInstance{
				Identifier: "mariadb-db",
				Engine:     "mariadb",
				Status:     "available",
				Port:       3306,
			},
			wantEngine: "mariadb",
			wantPort:   3306,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.rdsInstance.Engine != tt.wantEngine {
				t.Errorf("Expected Engine to be %s, got %s", tt.wantEngine, tt.rdsInstance.Engine)
			}
			if tt.rdsInstance.Port != tt.wantPort {
				t.Errorf("Expected Port to be %d, got %d", tt.wantPort, tt.rdsInstance.Port)
			}
		})
	}
}

func TestRDSEngineFiltering(t *testing.T) {
	tests := []struct {
		name           string
		engine         string
		shouldBeOracle bool
	}{
		{"Oracle standard", "oracle-ee", true},
		{"Oracle SE2", "oracle-se2", true},
		{"Oracle SE1", "oracle-se1", true},
		{"MySQL", "mysql", false},
		{"PostgreSQL", "postgres", false},
		{"MariaDB", "mariadb", false},
		{"Aurora MySQL", "aurora-mysql", false},
		{"Aurora PostgreSQL", "aurora-postgresql", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isOracle := strings.Contains(strings.ToLower(tt.engine), "oracle")
			if isOracle != tt.shouldBeOracle {
				t.Errorf("Engine %s: expected oracle=%v, got %v", tt.engine, tt.shouldBeOracle, isOracle)
			}
		})
	}
}

func TestInstanceStates(t *testing.T) {
	validStates := []string{"pending", "running", "stopping", "stopped", "shutting-down", "terminated"}

	for _, state := range validStates {
		inst := Instance{
			ID:    "i-test",
			State: state,
		}
		if inst.State != state {
			t.Errorf("Expected state %s, got %s", state, inst.State)
		}
	}
}

func TestRDSInstanceStatuses(t *testing.T) {
	validStatuses := []string{
		"available",
		"backing-up",
		"creating",
		"deleting",
		"failed",
		"maintenance",
		"modifying",
		"rebooting",
		"starting",
		"stopped",
		"stopping",
	}

	for _, status := range validStatuses {
		rds := RDSInstance{
			Identifier: "test-db",
			Status:     status,
		}
		if rds.Status != status {
			t.Errorf("Expected status %s, got %s", status, rds.Status)
		}
	}
}

func TestInstanceTypeValidation(t *testing.T) {
	instanceTypes := []string{
		"t3.micro",
		"t3.small",
		"t3.medium",
		"t3.large",
		"m5.xlarge",
		"c5.2xlarge",
		"r5.4xlarge",
	}

	for _, iType := range instanceTypes {
		inst := Instance{
			ID:           "i-test",
			InstanceType: iType,
		}
		if inst.InstanceType != iType {
			t.Errorf("Expected instance type %s, got %s", iType, inst.InstanceType)
		}
	}
}

func TestRDSPorts(t *testing.T) {
	tests := []struct {
		engine      string
		defaultPort int32
		description string
	}{
		{"mysql", 3306, "MySQL default port"},
		{"mariadb", 3306, "MariaDB default port"},
		{"postgres", 5432, "PostgreSQL default port"},
		{"aurora-mysql", 3306, "Aurora MySQL default port"},
		{"aurora-postgresql", 5432, "Aurora PostgreSQL default port"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			rds := RDSInstance{
				Engine: tt.engine,
				Port:   tt.defaultPort,
			}
			if rds.Port != tt.defaultPort {
				t.Errorf("Expected port %d for %s, got %d", tt.defaultPort, tt.engine, rds.Port)
			}
		})
	}
}

func TestEmptyInstanceFields(t *testing.T) {
	inst := Instance{}

	if inst.ID != "" {
		t.Error("New instance ID should be empty")
	}
	if inst.Name != "" {
		t.Error("New instance Name should be empty")
	}
	if inst.PrivateIP != "" {
		t.Error("New instance PrivateIP should be empty")
	}
	if inst.PublicIP != "" {
		t.Error("New instance PublicIP should be empty")
	}
}

func TestEmptyRDSInstanceFields(t *testing.T) {
	rds := RDSInstance{}

	if rds.Identifier != "" {
		t.Error("New RDS instance Identifier should be empty")
	}
	if rds.Endpoint != "" {
		t.Error("New RDS instance Endpoint should be empty")
	}
	if rds.Port != 0 {
		t.Error("New RDS instance Port should be 0")
	}
}

func TestInstanceWithAllFields(t *testing.T) {
	inst := Instance{
		ID:           "i-0123456789abcdef0",
		Name:         "production-web-server",
		PrivateIP:    "10.0.1.100",
		PublicIP:     "54.123.45.67",
		State:        "running",
		InstanceType: "t3.medium",
	}

	if inst.ID == "" {
		t.Error("Instance ID should not be empty")
	}
	if inst.Name == "" {
		t.Error("Instance Name should not be empty")
	}
	if inst.PrivateIP == "" {
		t.Error("Instance PrivateIP should not be empty")
	}
	if inst.PublicIP == "" {
		t.Error("Instance PublicIP should not be empty")
	}
	if inst.State != "running" {
		t.Errorf("Expected state running, got %s", inst.State)
	}
}

func TestRDSInstanceWithAllFields(t *testing.T) {
	rds := RDSInstance{
		Identifier: "production-postgres-db",
		Endpoint:   "prod-db.abc123xyz.us-east-1.rds.amazonaws.com",
		Port:       5432,
		Engine:     "postgres",
		Status:     "available",
	}

	if rds.Identifier == "" {
		t.Error("RDS Identifier should not be empty")
	}
	if rds.Endpoint == "" {
		t.Error("RDS Endpoint should not be empty")
	}
	if rds.Port == 0 {
		t.Error("RDS Port should not be 0")
	}
	if rds.Engine == "" {
		t.Error("RDS Engine should not be empty")
	}
	if rds.Status != "available" {
		t.Errorf("Expected status available, got %s", rds.Status)
	}
}

func TestInstanceIDFormats(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"Valid long instance ID", "i-0123456789abcdef0", true},
		{"Valid short instance ID 1", "i-12345678", true},
		{"Valid short instance ID 2", "i-abcdef01", true},
		{"Invalid prefix", "e-0123456789abcdef0", false},
		{"Too short", "i-abcdef", false},
		{"Too long", "i-0123456789abcdef01234", false},
		{"Missing prefix", "0123456789abcdef0", false},
		{"Empty ID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate instance ID format: i- prefix and 10 or 19 characters total
			// Old format: i- + 8 hex chars = 10 chars
			// New format: i- + 17 hex chars = 19 chars
			hasValidPrefix := strings.HasPrefix(tt.id, "i-")
			isValidLength := len(tt.id) == 10 || len(tt.id) == 19
			isValid := hasValidPrefix && isValidLength

			if isValid != tt.valid {
				t.Errorf("Instance ID %s validation = %v, expected %v", tt.id, isValid, tt.valid)
			}
		})
	}
}

func TestRDSEngineVersions(t *testing.T) {
	engines := []string{
		"postgres",
		"postgres14",
		"postgres15",
		"mysql",
		"mysql8.0",
		"mariadb",
		"mariadb10.6",
		"aurora-mysql",
		"aurora-postgresql",
	}

	for _, engine := range engines {
		rds := RDSInstance{
			Identifier: "test-db",
			Engine:     engine,
		}
		if rds.Engine == "" {
			t.Error("RDS Engine should not be empty")
		}
		// Verify oracle is not in the list
		if strings.Contains(strings.ToLower(rds.Engine), "oracle") {
			t.Errorf("Oracle engine %s should be filtered out", engine)
		}
	}
}

func TestValidateShell(t *testing.T) {
	tests := []struct {
		platform, shell string
		wantErr         bool
	}{
		{"linux", "/bin/bash", false},
		{"linux", "/usr/local/bin/fish", false},
		{"linux", "bash", true},
		{"linux", "/bin/bash -c id", true},
		{"linux", "/bin/sh;id", true},
		{"linux", "$(id)", true},
		{"linux", "", true},
		{"windows", "powershell.exe", false},
		{"windows", "cmd.exe", false},
		{"windows", `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, false},
		{"windows", `C:\Program Files\PowerShell\7\pwsh.exe`, true},
		{"windows", "cmd.exe /c calc.exe", true},
		{"windows", "powershell", true},
	}
	for _, tt := range tests {
		t.Run(tt.platform+"/"+tt.shell, func(t *testing.T) {
			if err := validateShell(tt.platform, tt.shell); (err != nil) != tt.wantErr {
				t.Errorf("validateShell(%q, %q) error = %v, wantErr %v", tt.platform, tt.shell, err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	write := func(t *testing.T, dir, body string) string {
		t.Helper()
		p := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	missing := filepath.Join(t.TempDir(), "absent.yaml")

	t.Run("no files gives defaults", func(t *testing.T) {
		cfg, err := loadConfig("", []string{missing})
		if err != nil || cfg.Linux.Shell != "/bin/bash" || cfg.Windows.Shell != "powershell.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("working directory config is ignored", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "linux:\n  shell: /bin/zsh\n")
		t.Chdir(dir)
		cfg, err := loadConfig("", []string{missing})
		if err != nil || cfg.Linux.Shell != "/bin/bash" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("first existing search path wins and keeps other default", func(t *testing.T) {
		a := write(t, t.TempDir(), "linux:\n  shell: /bin/zsh\n")
		b := write(t, t.TempDir(), "linux:\n  shell: /bin/sh\n")
		cfg, err := loadConfig("", []string{missing, a, b})
		if err != nil || cfg.Linux.Shell != "/bin/zsh" || cfg.Windows.Shell != "powershell.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("malformed file is an error, not a fallthrough", func(t *testing.T) {
		bad := write(t, t.TempDir(), "linux: [unclosed\n")
		good := write(t, t.TempDir(), "linux:\n  shell: /bin/sh\n")
		if _, err := loadConfig("", []string{bad, good}); err == nil || !strings.Contains(err.Error(), bad) {
			t.Fatalf("want error naming %s, got %v", bad, err)
		}
	})
	t.Run("invalid shell is an error", func(t *testing.T) {
		p := write(t, t.TempDir(), "linux:\n  shell: /bin/bash -c id\n")
		if _, err := loadConfig("", []string{p}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("explicit path is used and must exist", func(t *testing.T) {
		p := write(t, t.TempDir(), "windows:\n  shell: cmd.exe\n")
		cfg, err := loadConfig(p, []string{missing})
		if err != nil || cfg.Windows.Shell != "cmd.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
		if _, err := loadConfig(missing, nil); err == nil {
			t.Fatal("want error for missing explicit path")
		}
	})
}

func TestDefaultConfigPaths(t *testing.T) {
	check := func(t *testing.T) {
		for _, p := range defaultConfigPaths() {
			if !filepath.IsAbs(p) {
				t.Errorf("relative config path %q", p)
			}
		}
	}
	t.Run("with HOME", func(t *testing.T) { t.Setenv("HOME", t.TempDir()); check(t) })
	t.Run("HOME unset", func(t *testing.T) { t.Setenv("HOME", ""); check(t) })
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"admin":   `'admin'`,
		"o'brien": `'o'\''brien'`,
		"a b;id":  `'a b;id'`,
		"":        `''`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestRDSConnectionExamples(t *testing.T) {
	join := func(engine, user string) string {
		return strings.Join(rdsConnectionExamples(engine, "db.example.internal", 5432, user), "\n")
	}
	t.Run("postgres verifies the certificate", func(t *testing.T) {
		out := join("aurora-postgresql", "app")
		for _, want := range []string{"sslmode=verify-full", "sslrootcert=./global-bundle.pem", `PGPASSWORD="$RDS_TOKEN"`, "host=db.example.internal", "port=5432"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "sslmode=require") {
			t.Error("unverified sslmode present")
		}
	})
	t.Run("mysql verifies the certificate", func(t *testing.T) {
		out := join("mariadb", "app")
		for _, want := range []string{"--ssl-mode=VERIFY_IDENTITY", "--ssl-ca=./global-bundle.pem", `MYSQL_PWD="$RDS_TOKEN"`, "-u 'app'"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "--password") || strings.Contains(out, "REQUIRED") {
			t.Error("password argument or unverified ssl-mode present")
		}
	})
	t.Run("username with a quote is escaped", func(t *testing.T) {
		if out := join("mysql", "o'brien"); !strings.Contains(out, `'o'\''brien'`) {
			t.Errorf("unescaped username in:\n%s", out)
		}
		if out := join("postgres", "o'brien"); !strings.Contains(out, `-U 'o'\''brien'`) || !strings.Contains(out, "PGSSLMODE=verify-full") {
			t.Errorf("want -U form with quoted user, got:\n%s", out)
		}
	})
	t.Run("unknown engine yields no commands", func(t *testing.T) {
		if got := rdsConnectionExamples("sqlserver-se", "h", 1433, "sa"); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
}
