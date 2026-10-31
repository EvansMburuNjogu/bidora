package localprofile_test

import (
	"strings"
	"testing"

	"bidora/backend/internal/config"
	"bidora/backend/internal/localprofile"
)

func localEnvironment() map[string]string {
	return map[string]string{
		"BIDORA_ENV":               "local",
		"BIDORA_PROFILE":           "postgres-local",
		"BIDORA_LOCAL_CPUS":        "2",
		"BIDORA_LOCAL_MEMORY_MIB":  "2048",
		"BIDORA_LOCAL_STORAGE_GIB": "10",
	}
}

func TestLoadExplicitLocalSettings(t *testing.T) {
	for _, env := range []string{"local", "test"} {
		for _, minimum := range []bool{false, true} {
			values := localEnvironment()
			values["BIDORA_ENV"] = env
			want := validConfig()
			want.Environment = env
			if minimum {
				values["BIDORA_LOCAL_CPUS"] = "1"
				values["BIDORA_LOCAL_MEMORY_MIB"] = "1"
				values["BIDORA_LOCAL_STORAGE_GIB"] = "1"
				want.CPUs, want.MemoryMiB, want.StorageGiB = 1, 1, 1
			}
			reads := map[string]int{}
			got, err := localprofile.Load(func(key string) string {
				reads[key]++
				return values[key]
			})
			if err != nil || got != want {
				t.Fatalf("Load() = %+v, %v; want %+v", got, err, want)
			}
			if len(reads) != len(values) {
				t.Fatalf("unexpected environment keys read: %v", reads)
			}
			for key := range values {
				if reads[key] != 1 {
					t.Fatalf("%s read %d times", key, reads[key])
				}
			}
		}
	}
}

func TestLoadRejectsMalformedResources(t *testing.T) {
	const canary = "secret-canary"
	for _, key := range []string{"BIDORA_LOCAL_CPUS", "BIDORA_LOCAL_MEMORY_MIB", "BIDORA_LOCAL_STORAGE_GIB"} {
		t.Run(key, func(t *testing.T) {
			for _, value := range []string{"", "0", "-1", "+1", "01", " 1", "1 ", "1\n", "1\x00", "1.5", "1e0", "0x1", "1_000", "１", "1x", "99999999999999999999", strings.Repeat("9", 10000), canary} {
				values := localEnvironment()
				values[key] = value
				got, err := localprofile.Load(func(k string) string { return values[k] })
				if err == nil || got != (localprofile.Config{}) {
					t.Fatalf("invalid %s accepted or partial config returned", key)
				}
				wantError := key + " must be an explicit positive decimal integer"
				if err.Error() != wantError || strings.Contains(err.Error(), canary) {
					t.Fatalf("unexpected or unsafe parsing error: %v", err)
				}
			}
		})
	}
}

func TestLoadRejectsMissingFields(t *testing.T) {
	for key := range localEnvironment() {
		t.Run(key, func(t *testing.T) {
			values := localEnvironment()
			delete(values, key)
			got, err := localprofile.Load(func(k string) string { return values[k] })
			if err == nil || got != (localprofile.Config{}) {
				t.Fatal("missing field accepted or partial configuration returned")
			}
		})
	}
}

func TestLoadValidatesScopeAndCeilings(t *testing.T) {
	for _, tt := range []struct{ key, value string }{
		{"BIDORA_ENV", "production"}, {"BIDORA_ENV", "LOCAL"}, {"BIDORA_ENV", "local "},
		{"BIDORA_PROFILE", "health-only"}, {"BIDORA_PROFILE", "POSTGRES-LOCAL"}, {"BIDORA_PROFILE", " postgres-local"},
		{"BIDORA_ENV", "secret-scope-canary"}, {"BIDORA_PROFILE", "secret-scope-canary"},
		{"BIDORA_LOCAL_CPUS", "3"}, {"BIDORA_LOCAL_MEMORY_MIB", "2049"}, {"BIDORA_LOCAL_STORAGE_GIB", "11"},
	} {
		values := localEnvironment()
		values[tt.key] = tt.value
		got, err := localprofile.Load(func(k string) string { return values[k] })
		if err == nil || got != (localprofile.Config{}) {
			t.Fatalf("invalid %s accepted or partial configuration returned", tt.key)
		}
		if strings.Contains(err.Error(), "secret-scope-canary") {
			t.Fatal("scope input leaked")
		}
	}
}

func TestLoadNilReader(t *testing.T) {
	got, err := localprofile.Load(nil)
	if err == nil || got != (localprofile.Config{}) {
		t.Fatal("nil reader accepted")
	}
}

func TestLoadDoesNotReadAmbientEnvironment(t *testing.T) {
	t.Setenv("BIDORA_ENV", "production")
	t.Setenv("BIDORA_LOCAL_CPUS", "999")
	values := localEnvironment()
	got, err := localprofile.Load(func(k string) string { return values[k] })
	if err != nil || got != validConfig() {
		t.Fatalf("ambient environment affected explicit reader: %+v, %v", got, err)
	}
}

func TestHealthProfileRemainsSeparate(t *testing.T) {
	values := localEnvironment()
	getenv := func(k string) string { return values[k] }
	if _, err := config.Load(getenv); err == nil {
		t.Fatal("existing API configuration accepted postgres-local")
	}
	values["BIDORA_PROFILE"] = "health-only"
	delete(values, "BIDORA_LOCAL_CPUS")
	delete(values, "BIDORA_LOCAL_MEMORY_MIB")
	delete(values, "BIDORA_LOCAL_STORAGE_GIB")
	if _, err := config.Load(getenv); err != nil {
		t.Fatalf("health-only now requires local resource fields: %v", err)
	}
}
