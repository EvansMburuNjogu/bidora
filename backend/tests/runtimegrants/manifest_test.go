package runtimegrants_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"bidora/backend/internal/runtimegrants"
)

var (
	createTable = regexp.MustCompile(`(?is)CREATE\s+(?:OR\s+REPLACE\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
	insertInto  = regexp.MustCompile(`(?is)\bINSERT\s+INTO\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
	updateTable = regexp.MustCompile(`(?is)\bUPDATE\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
	deleteFrom  = regexp.MustCompile(`(?is)\bDELETE\s+FROM\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
	qualified   = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)\b`)
	returning   = regexp.MustCompile(`(?is)\bRETURNING\b`)
)

func TestRuntimeGrantManifestsExactlyCoverReviewedQueries(t *testing.T) {
	root := backendRoot(t)
	known := knownTables(t, root)
	for _, tc := range []struct {
		plane string
		dirs  []string
		count int
	}{{"control", []string{"control", "identity", "controlplane", "platformadmin", "platformsettings", "commercial", "authorization", "notification"}, 97}, {"tenant", []string{"eventing", "organization", "tenantevidence", "entitlement", "opportunity"}, 61}} {
		t.Run(tc.plane, func(t *testing.T) {
			manifest, err := runtimegrants.Load(tc.plane)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]map[runtimegrants.Privilege]bool{}
			for _, grant := range manifest.Tables {
				got[grant.Table] = map[runtimegrants.Privilege]bool{}
				for _, privilege := range grant.Privileges {
					got[grant.Table][privilege] = true
				}
			}
			want := queryPrivileges(t, root, known, tc.dirs)
			if len(got) != tc.count || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s runtime grant manifest drifted\n got=%v\nwant=%v", tc.plane, sortedPermissions(got), sortedPermissions(want))
			}
		})
	}
}

func TestRuntimeGrantManifestMetadataIsFixed(t *testing.T) {
	control, err := runtimegrants.Load("control")
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := runtimegrants.Load("tenant")
	if err != nil {
		t.Fatal(err)
	}
	if runtimegrants.Version != 1 || control.CompatibilitySet != "control_registry" || control.CompatibilityVersion != 50 || tenant.CompatibilitySet != "tenant_common+tenant_tenderpreneur" || tenant.CompatibilityVersion != 9_000_034 {
		t.Fatal("runtime grant manifest compatibility metadata drifted")
	}
	if _, err = runtimegrants.Load("exchange"); err == nil {
		t.Fatal("unknown runtime grant plane accepted")
	}
}

func TestControlRuntimeGrantsPreserveAuditBoundaryAndRegistration(t *testing.T) {
	manifest, err := runtimegrants.Load("control")
	if err != nil {
		t.Fatal(err)
	}
	grants := map[string][]runtimegrants.Privilege{}
	for _, grant := range manifest.Tables {
		grants[grant.Table] = grant.Privileges
	}
	for table, want := range map[string][]runtimegrants.Privilege{
		"bidora_control_audit.audit_events":             {runtimegrants.Insert},
		"bidora_control_audit.platform_action_details_v1": {runtimegrants.Select},
		"bidora_tenancy.tenants":                        {runtimegrants.Select, runtimegrants.Insert},
		"tenancy.provisioning_jobs":                     {runtimegrants.Insert},
	} {
		got := append([]runtimegrants.Privilege(nil), grants[table]...)
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s privileges: got %v, want %v", table, got, want)
		}
	}
}

func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func knownTables(t *testing.T, root string) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	files, err := filepath.Glob(filepath.Join(root, "migrations", "*", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range createTable.FindAllStringSubmatch(string(raw), -1) {
			result[match[1]] = true
		}
	}
	return result
}

func queryPrivileges(t *testing.T, root string, known map[string]bool, dirs []string) map[string]map[runtimegrants.Privilege]bool {
	t.Helper()
	result := map[string]map[runtimegrants.Privilege]bool{}
	add := func(table string, privilege runtimegrants.Privilege) {
		if !known[table] {
			return
		}
		if result[table] == nil {
			result[table] = map[runtimegrants.Privilege]bool{}
		}
		result[table][privilege] = true
	}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(root, "queries", dir, "*.sql"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			chunks := strings.Split("\n"+string(raw), "\n-- name:")
			for _, chunk := range chunks[1:] {
				lines := strings.Split(chunk, "\n")
				if len(lines) < 2 {
					continue
				}
				query := strings.Join(lines[1:], "\n")
				scan := query
				hasReturning := returning.MatchString(query)
				for _, match := range insertInto.FindAllStringSubmatch(query, -1) {
					add(match[1], runtimegrants.Insert)
					if hasReturning {
						add(match[1], runtimegrants.Select)
					}
				}
				scan = insertInto.ReplaceAllString(scan, "mutation_target")
				for _, patternPrivilege := range []struct {
					pattern   *regexp.Regexp
					privilege runtimegrants.Privilege
				}{{updateTable, runtimegrants.Update}, {deleteFrom, runtimegrants.Delete}} {
					for _, match := range patternPrivilege.pattern.FindAllStringSubmatch(query, -1) {
						add(match[1], patternPrivilege.privilege)
						add(match[1], runtimegrants.Select)
					}
					scan = patternPrivilege.pattern.ReplaceAllString(scan, "mutation_target")
				}
				for _, match := range qualified.FindAllStringSubmatch(scan, -1) {
					add(match[1], runtimegrants.Select)
				}
			}
		}
	}
	return result
}

func sortedPermissions(values map[string]map[runtimegrants.Privilege]bool) []string {
	result := make([]string, 0, len(values))
	for table, privileges := range values {
		list := make([]string, 0, len(privileges))
		for privilege := range privileges {
			list = append(list, string(privilege))
		}
		sort.Strings(list)
		result = append(result, table+":"+strings.Join(list, ","))
	}
	sort.Strings(result)
	return result
}
