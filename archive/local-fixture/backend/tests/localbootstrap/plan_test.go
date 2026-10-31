package localbootstrap_test

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/localbootstrap"
	"bidora/backend/internal/localprofile"
)

func fixtureConfig() localprofile.PostgresConfig {
	return localprofile.PostgresConfig{
		Resources: localprofile.Config{Environment: "local", Profile: "postgres-local", CPUs: 2, MemoryMiB: 2048, StorageGiB: 10},
		Address:   netip.MustParseAddrPort("127.0.0.1:15432"),
	}
}

func TestPlanFixedMappingAndBoundaries(t *testing.T) {
	config := fixtureConfig()
	before := config
	plan, err := localbootstrap.BuildPlan(config)
	if err != nil || len(plan.Steps) != 16 || config != before {
		t.Fatal("plan shape or input preservation failed")
	}
	roles := plan.Steps[0]
	if roles.ID != "roles" || roles.Database != "postgres" || roles.LoginRole != "bidora_local_bootstrap" || !roles.Transactional || len(roles.Statements) != 18 {
		t.Fatal("unexpected role setup boundary")
	}
	setup := strings.Join(roles.Statements, "\n")
	for _, required := range []string{
		`CREATE ROLE "bidora_local_provisioner" NOLOGIN NOINHERIT NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS;`,
		`REVOKE CONNECT, TEMPORARY ON DATABASE "postgres" FROM PUBLIC;`,
		`REVOKE CONNECT, TEMPORARY ON DATABASE "template1" FROM PUBLIC;`,
	} {
		if !strings.Contains(setup, required) {
			t.Fatal("missing reviewed administrative boundary")
		}
	}
	for i, name := range []string{"control", "tenant_a", "tenant_b"} {
		db := "bidora_local_" + name
		owner, migrator, runtime := db+"_owner", db+"_migrator", db+"_runtime"
		for role, limit := range map[string]string{owner: "", migrator: " CONNECTION LIMIT 1", runtime: " CONNECTION LIMIT 2"} {
			want := `CREATE ROLE "` + role + `" NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS` + limit + ";"
			if !strings.Contains(setup, want) {
				t.Fatal("role privilege flags differ from review")
			}
		}
		if !strings.Contains(setup, `GRANT "`+owner+`" TO "`+migrator+`";`) || strings.Contains(setup, `TO "`+runtime+`"`) {
			t.Fatal("owner membership leaked to runtime or omitted for migrator")
		}
		steps := plan.Steps[1+i*5 : 1+(i+1)*5]
		create, grants, schema := steps[0], steps[1], steps[4]
		if create.Transactional || create.EffectiveRole != "bidora_local_provisioner" || create.LoginRole != "bidora_local_bootstrap" || create.Database != "postgres" || !reflect.DeepEqual(create.Statements, []string{`CREATE DATABASE "` + db + `" OWNER "` + owner + `" TEMPLATE template0;`}) {
			t.Fatal("CREATE DATABASE must be explicit, owned, nontransactional provisioner work")
		}
		wantGrants := []string{
			`REVOKE ALL ON DATABASE "` + db + `" FROM PUBLIC;`,
			`GRANT CONNECT ON DATABASE "` + db + `" TO "` + migrator + `", "` + runtime + `";`,
			`REVOKE "` + owner + `" FROM "bidora_local_provisioner";`,
		}
		if !grants.Transactional || !reflect.DeepEqual(grants.Statements, wantGrants) {
			t.Fatal("database ACL plan differs from reviewed scope")
		}
		for j, role := range []string{migrator, runtime} {
			step := steps[2+j]
			if step.Kind != localbootstrap.CredentialStep || step.CredentialRole != role || len(step.Statements) != 0 || !step.Transactional || step.LoginRole != "bidora_local_bootstrap" {
				t.Fatal("credential activation must be separate, explicit and administrator-owned")
			}
		}
		if schema.Database != db || schema.LoginRole != migrator || schema.EffectiveRole != owner || !schema.Transactional {
			t.Fatal("schema must be created by its intended owner through its migrator")
		}
		wantSchema := []string{
			`REVOKE ALL ON SCHEMA public FROM PUBLIC;`,
			`CREATE SCHEMA bidora_fixture AUTHORIZATION "` + owner + `";`,
			`REVOKE ALL ON SCHEMA bidora_fixture FROM PUBLIC;`,
			`CREATE TABLE bidora_fixture.sentinel (
    probe_key text PRIMARY KEY CHECK (octet_length(probe_key) BETWEEN 1 AND 64),
    probe_value text NOT NULL CHECK (octet_length(probe_value) <= 256)
);`,
			`REVOKE ALL ON TABLE bidora_fixture.sentinel FROM PUBLIC;`,
			`GRANT USAGE ON SCHEMA bidora_fixture TO "` + runtime + `";`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE bidora_fixture.sentinel TO "` + runtime + `";`,
		}
		if !reflect.DeepEqual(schema.Statements, wantSchema) {
			t.Fatal("fixture schema/grants differ from review")
		}
	}
	seen := map[string]bool{}
	for _, step := range plan.Steps {
		if seen[step.ID] || step.ID == "" || step.StatementTimeout != 5*time.Second || step.LockTimeout != time.Second {
			t.Fatal("duplicate checkpoint identity or missing deadline")
		}
		seen[step.ID] = true
		for _, statement := range step.Statements {
			for _, prohibited := range []string{"PASSWORD", " LOGIN", "DROP ", "IF NOT EXISTS", "GRANT ALL", "ALTER DEFAULT PRIVILEGES", "dblink", "BEGIN;", "COMMIT;"} {
				if strings.Contains(statement, prohibited) {
					t.Fatalf("unexpected plan SQL capability: %s", prohibited)
				}
			}
		}
	}
}

func TestPlanRejectsInvalidScopeAndOtherEndpoints(t *testing.T) {
	for _, mutate := range []func(*localprofile.PostgresConfig){
		func(c *localprofile.PostgresConfig) { c.Resources.Environment = "production" },
		func(c *localprofile.PostgresConfig) { c.Resources.Profile = "health-only" },
		func(c *localprofile.PostgresConfig) { c.Resources.CPUs = 3 },
		func(c *localprofile.PostgresConfig) { c.Resources.MemoryMiB = 2049 },
		func(c *localprofile.PostgresConfig) { c.Resources.StorageGiB = 11 },
		func(c *localprofile.PostgresConfig) { c.Address = netip.AddrPort{} },
		func(c *localprofile.PostgresConfig) { c.Address = netip.MustParseAddrPort("127.0.0.1:5432") },
		func(c *localprofile.PostgresConfig) { c.Address = netip.MustParseAddrPort("127.0.0.2:15432") },
		func(c *localprofile.PostgresConfig) { c.Address = netip.MustParseAddrPort("[::1]:15432") },
	} {
		c := fixtureConfig()
		mutate(&c)
		if plan, err := localbootstrap.BuildPlan(c); err == nil || len(plan.Steps) != 0 {
			t.Fatal("invalid bootstrap scope accepted or partial plan returned")
		}
	}
	c := fixtureConfig()
	c.Resources.Environment = "test"
	if _, err := localbootstrap.BuildPlan(c); err != nil {
		t.Fatal("explicit test scope rejected")
	}
}

func TestPlanHasNoSharedMutableState(t *testing.T) {
	first, err := localbootstrap.BuildPlan(fixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	first.Steps[0].Statements[0] = "corrupted"
	first.Steps[1].Database = "unrelated"
	second, err := localbootstrap.BuildPlan(fixtureConfig())
	if err != nil || strings.Contains(second.Steps[0].Statements[0], "corrupted") || second.Steps[1].Database != "postgres" {
		t.Fatal("caller mutation contaminated future plans")
	}
}
