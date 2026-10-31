package localprofile_test

import (
	"math"
	"strings"
	"testing"

	"bidora/backend/internal/localprofile"
)

func validConfig() localprofile.Config {
	return localprofile.Config{
		Environment: "local",
		Profile:     "postgres-local",
		CPUs:        2,
		MemoryMiB:   2048,
		StorageGiB:  10,
	}
}

func TestValidateValidBoundaries(t *testing.T) {
	for _, env := range []string{"local", "test"} {
		for _, cpu := range []int{1, 2} {
			for _, memory := range []int{1, 1024, 2048} {
				for _, storage := range []int{1, 5, 10} {
					c := localprofile.Config{Environment: env, Profile: "postgres-local", CPUs: cpu, MemoryMiB: memory, StorageGiB: storage}
					before := c
					if err := c.Validate(); err != nil {
						t.Fatalf("valid config %+v rejected: %v", c, err)
					}
					if c != before {
						t.Fatal("validation mutated valid input")
					}
				}
			}
		}
	}
}

func TestValidateRejectsInvalidScope(t *testing.T) {
	const canary = "synthetic-private-value-DO-NOT-ECHO"
	fields := []struct {
		name   string
		values []string
		set    func(*localprofile.Config, string)
		want   string
	}{
		{"environment", []string{"", "production", "staging", "LOCAL", "TEST", " local", "test ", "local\n", "local\x00", canary}, func(c *localprofile.Config, v string) { c.Environment = v }, "environment must be local or test"},
		{"profile", []string{"", "health-only", "postgres", "POSTGRES-LOCAL", " postgres-local", "postgres-local ", "postgres-local\n", "postgres-local\x00", canary}, func(c *localprofile.Config, v string) { c.Profile = v }, "profile must be postgres-local"},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			for _, value := range field.values {
				c := validConfig()
				field.set(&c, value)
				before := c
				err := c.Validate()
				if err == nil || err.Error() != field.want {
					t.Fatalf("unexpected error for invalid %s: %v", field.name, err)
				}
				if strings.Contains(err.Error(), canary) {
					t.Fatal("validation error echoed private input")
				}
				if c != before {
					t.Fatal("validation mutated invalid scope")
				}
			}
		})
	}
}

func TestValidateRejectsInvalidResources(t *testing.T) {
	fields := []struct {
		name string
		max  int
		set  func(*localprofile.Config, int)
		want string
	}{
		{"CPUs", 2, func(c *localprofile.Config, v int) { c.CPUs = v }, "CPUs must be between 1 and 2"},
		{"memory", 2048, func(c *localprofile.Config, v int) { c.MemoryMiB = v }, "memory MiB must be between 1 and 2048"},
		{"storage", 10, func(c *localprofile.Config, v int) { c.StorageGiB = v }, "storage GiB must be between 1 and 10"},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			for _, value := range []int{math.MinInt, -1, 0, field.max + 1, math.MaxInt} {
				c := validConfig()
				field.set(&c, value)
				before := c
				err := c.Validate()
				if err == nil || err.Error() != field.want {
					t.Fatalf("unexpected error for %s=%d: %v", field.name, value, err)
				}
				if c != before {
					t.Fatal("invalid resource was defaulted or clamped")
				}
			}
		})
	}
}

func TestValidateMultipleInvalidFields(t *testing.T) {
	cases := []localprofile.Config{
		{},
		{Environment: "production", Profile: "other", CPUs: -1, MemoryMiB: -1, StorageGiB: -1},
		{Environment: "local", Profile: "postgres-local", CPUs: 3, MemoryMiB: 2049, StorageGiB: 11},
	}
	for _, c := range cases {
		before := c
		if err := c.Validate(); err == nil {
			t.Fatal("multiple invalid fields accepted")
		}
		if c != before {
			t.Fatal("invalid input mutated")
		}
	}
}
