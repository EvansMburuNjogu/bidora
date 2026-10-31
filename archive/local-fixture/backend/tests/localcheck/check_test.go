package localcheck_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"bidora/backend/internal/localcheck"
)

func settings() map[string]string {
	return map[string]string{
		"BIDORA_ENV": "local", "BIDORA_PROFILE": "postgres-local",
		"BIDORA_LOCAL_CPUS": "2", "BIDORA_LOCAL_MEMORY_MIB": "2048", "BIDORA_LOCAL_STORAGE_GIB": "10",
		"BIDORA_PG_ADDR": "127.0.0.1:15432",
	}
}

func TestRunValidConfiguration(t *testing.T) {
	values := settings()
	var stdout, stderr bytes.Buffer
	code := localcheck.Run(func(key string) string { return values[key] }, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d, stderr=%q", code, stderr.String())
	}
	if stdout.String() != "Local configuration valid; database connectivity, credentials, roles and resource enforcement NOT checked.\n" {
		t.Fatalf("misleading preflight result: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), values["BIDORA_PG_ADDR"]) {
		t.Fatal("preflight unnecessarily disclosed configuration")
	}
}

func TestRunInvalidConfiguration(t *testing.T) {
	for _, mutate := range []func(map[string]string){
		func(v map[string]string) { delete(v, "BIDORA_PG_ADDR") },
		func(v map[string]string) { v["BIDORA_PG_ADDR"] = "postgres://secret-canary@host/db" },
		func(v map[string]string) { v["BIDORA_ENV"] = "production" },
		func(v map[string]string) { v["BIDORA_LOCAL_CPUS"] = "3" },
	} {
		values := settings()
		mutate(values)
		var stdout, stderr bytes.Buffer
		code := localcheck.Run(func(key string) string { return values[key] }, &stdout, &stderr)
		if code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("code=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "secret-canary") {
			t.Fatal("error leaked input")
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("secret-writer-error") }

func TestRunOutputFailure(t *testing.T) {
	values := settings()
	var stderr bytes.Buffer
	code := localcheck.Run(func(key string) string { return values[key] }, failedWriter{}, &stderr)
	if code != 1 || stderr.String() != "local-check: unable to write result\n" {
		t.Fatalf("output error not handled safely: code=%d, stderr=%q", code, stderr.String())
	}
}

func TestRunErrorOutputFailure(t *testing.T) {
	var stdout bytes.Buffer
	if code := localcheck.Run(nil, &stdout, failedWriter{}); code != 1 || stdout.Len() != 0 {
		t.Fatal("failed diagnostics must not turn an invalid configuration into success")
	}
}
