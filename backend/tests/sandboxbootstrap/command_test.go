package sandboxbootstrap_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxOwnerCommandFailsClosedWithoutNetwork(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "sandbox-owner-bootstrap")
	build := exec.Command("go", "build", "-o", binary, "./cmd/sandbox-owner-bootstrap")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", output)
	}
	tests := []struct {
		name string
		args []string
		env  []string
		want string
	}{
		{"unknown phase", []string{"activate-smtp", "unused"}, nil, "bootstrap_arguments_invalid"},
		{"no environment", []string{"promote", "unused"}, nil, "bootstrap_environment_not_sandbox"},
		{"production denied", []string{"verify-email", "unused"}, []string{"BIDORA_ENV=production"}, "bootstrap_environment_not_sandbox"},
		{"wrong database", []string{"promote", "unused"}, []string{"BIDORA_ENV=staging", "BIDORA_TEST_CONTROL_RUNTIME_DSN=postgres://bidora_sandbox_runtime:do-not-log-this@postgres:5432/production?sslmode=verify-full"}, "bootstrap_database_configuration_not_sandbox"},
		{"wrong role", []string{"promote", "unused"}, []string{"BIDORA_ENV=staging", "BIDORA_TEST_CONTROL_RUNTIME_DSN=postgres://postgres:do-not-log-this@postgres:5432/bidora_sandbox_control?sslmode=verify-full"}, "bootstrap_database_configuration_not_sandbox"},
		{"unencrypted database denied", []string{"promote", "unused"}, []string{"BIDORA_ENV=staging", "BIDORA_TEST_CONTROL_RUNTIME_DSN=postgres://bidora_sandbox_runtime:do-not-log-this@postgres:5432/bidora_sandbox_control?sslmode=disable"}, "bootstrap_database_configuration_not_sandbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(binary, tt.args...)
			command.Env = append([]string{"PATH=" + os.Getenv("PATH")}, tt.env...)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatal("unsafe command unexpectedly succeeded")
			}
			if strings.TrimSpace(string(output)) != tt.want {
				t.Fatalf("unexpected stable error: %q", output)
			}
			if strings.Contains(string(output), "do-not-log-this") {
				t.Fatal("credential leaked")
			}
		})
	}
}
