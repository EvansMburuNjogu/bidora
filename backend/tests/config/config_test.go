package config_test

import (
	"bidora/backend/internal/config"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name, profile, env, addr string
		valid                    bool
	}{
		{"defaults", "health-only", "local", "", true},
		{"test", "health-only", "test", "127.0.0.1:8091", true},
		{"ipv6", "health-only", "local", "[::1]:8080", true},
		{"missing profile", "", "local", "", false},
		{"missing environment", "health-only", "", "", false},
		{"production", "health-only", "production", "", false},
		{"public bind", "health-only", "local", "0.0.0.0:8080", false},
		{"invalid port", "health-only", "local", "127.0.0.1:0", false},
		{"invalid host", "health-only", "local", "secret-token.invalid:8080", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{"BIDORA_PROFILE": tt.profile, "BIDORA_ENV": tt.env, "BIDORA_HTTP_ADDR": tt.addr}
			c, err := config.Load(func(k string) string { return values[k] })
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-token") {
				t.Fatal("config value leaked")
			}
			if tt.valid && c.Address == "" {
				t.Fatal("empty address")
			}
		})
	}
}
