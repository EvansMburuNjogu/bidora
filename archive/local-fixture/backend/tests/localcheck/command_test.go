package localcheck_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"bidora/backend/internal/localcheck"
)

func TestRunCommandArgumentBoundary(t *testing.T) {
	for _, args := range [][]string{{"--unknown-secret-canary"}, {"--tools", "extra"}} {
		var stdout, stderr bytes.Buffer
		code := localcheck.RunCommand(context.Background(), args, func(string) string {
			t.Fatal("invalid arguments read configuration")
			return ""
		}, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || stderr.String() != "usage: local-check [--tools]\n" {
			t.Fatal("unknown arguments were ignored or echoed")
		}
	}
	values := settings()
	var stdout, stderr bytes.Buffer
	if code := localcheck.RunCommand(context.Background(), nil, func(key string) string { return values[key] }, &stdout, &stderr); code != 0 {
		t.Fatal("default configuration-only mode regressed")
	}
}

func TestRunToolsRequiresLocalConfiguration(t *testing.T) {
	values := settings()
	values["BIDORA_ENV"] = "production"
	var stdout, stderr bytes.Buffer
	code := localcheck.RunCommand(context.Background(), []string{"--tools"}, func(key string) string {
		if key == "BIDORA_PG_BIN_DIR" || key == "BIDORA_PG_VERSION" {
			t.Fatal("tool inspection attempted before local configuration validation")
		}
		return values[key]
	}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 {
		t.Fatal("non-local tool invocation accepted")
	}
}

func TestRunToolsRequiresExplicitDirectoryAndVersion(t *testing.T) {
	values := settings()
	var stdout, stderr bytes.Buffer
	code := localcheck.RunCommand(context.Background(), []string{"--tools"}, func(key string) string { return values[key] }, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatal("missing tool settings accepted")
	}
}

func TestRunToolsReportsOnlyVersionMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Unix tool fixture")
	}
	dir := t.TempDir()
	for _, tool := range []string{"postgres", "pg_ctl", "initdb", "psql"} {
		if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nprintf '"+tool+" (PostgreSQL) 15.17\\n'\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	values := settings()
	values["BIDORA_PG_BIN_DIR"] = dir
	values["BIDORA_PG_VERSION"] = "15.17"
	getenv := func(key string) string { return values[key] }
	var stdout, stderr bytes.Buffer
	if code := localcheck.RunCommand(context.Background(), []string{"--tools"}, getenv, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("valid tool check failed: %d, %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tools match 15.17") || !strings.Contains(stdout.String(), "NOT checked") || strings.Contains(stdout.String(), dir) {
		t.Fatalf("misleading or excessive output: %s", stdout.String())
	}
	if code := localcheck.RunCommand(context.Background(), []string{"--tools"}, getenv, failedWriter{}, &stderr); code != 1 {
		t.Fatal("output failure reported as success")
	}
}
