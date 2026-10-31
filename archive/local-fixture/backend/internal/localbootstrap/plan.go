// Package localbootstrap prepares the approved synthetic native fixture only.
// It contains no SQL executor, PostgreSQL cluster provisioning or process startup.
package localbootstrap

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"bidora/backend/internal/localprofile"
)

// StepKind distinguishes SQL from credential activation; an empty SQL list must
// never be mistaken for completed credential setup.
type StepKind string

const (
	SQLStep        StepKind = "sql"
	CredentialStep StepKind = "credential_activation"
)

// Step describes a proposed operation, not proof of permissions or ownership.
// A future executor must revalidate owned state/checkpoints before every effect.
// Statements contain no passwords/verifiers. Credential steps require separate,
// in-memory rendering after role attributes and grants have been verified.
type Step struct {
	ID               string
	Kind             StepKind
	Database         string
	LoginRole        string
	EffectiveRole    string
	CredentialRole   string
	Transactional    bool
	StatementTimeout time.Duration
	LockTimeout      time.Duration
	Statements       []string
}

// Plan is freshly allocated on every build. It is not an executable or trusted
// serialized command format: callers must not execute edited/deserialized plans.
type Plan struct {
	Steps []Step
}

// BuildPlan produces only the fixed, reviewed synthetic database/role mapping.
// The narrower fixture exception permits exactly 127.0.0.1:15432, not arbitrary
// endpoints accepted by the general configuration-only local preflight.
func BuildPlan(config localprofile.PostgresConfig) (Plan, error) {
	if err := config.Resources.Validate(); err != nil {
		return Plan{}, err
	}
	if config.Address != netip.MustParseAddrPort("127.0.0.1:15432") {
		return Plan{}, errors.New("bootstrap fixture requires its fixed loopback endpoint")
	}
	setup := newStep("roles", "postgres", "bidora_local_bootstrap", "bidora_local_bootstrap", true)
	setup.Statements = []string{
		`CREATE ROLE "bidora_local_provisioner" NOLOGIN NOINHERIT NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS;`,
		`REVOKE CONNECT, TEMPORARY ON DATABASE "postgres" FROM PUBLIC;`,
		`REVOKE CONNECT, TEMPORARY ON DATABASE "template1" FROM PUBLIC;`,
	}
	for _, target := range targets() {
		for _, entry := range []struct {
			name  string
			limit string
		}{{target.owner, ""}, {target.migrator, " CONNECTION LIMIT 1"}, {target.runtime, " CONNECTION LIMIT 2"}} {
			setup.Statements = append(setup.Statements, fmt.Sprintf(
				"CREATE ROLE %s NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS%s;",
				quote(entry.name), entry.limit))
		}
		setup.Statements = append(setup.Statements,
			fmt.Sprintf("GRANT %s TO %s;", quote(target.owner), quote(target.migrator)),
			fmt.Sprintf("GRANT %s TO \"bidora_local_provisioner\";", quote(target.owner)))
	}
	plan := Plan{Steps: []Step{setup}}
	for _, target := range targets() {
		create := newStep(target.name+".create_database", "postgres", "bidora_local_bootstrap", "bidora_local_provisioner", false)
		create.Statements = []string{fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE template0;", quote(target.database), quote(target.owner))}
		grants := newStep(target.name+".grants", "postgres", "bidora_local_bootstrap", "bidora_local_bootstrap", true)
		grants.Statements = []string{
			fmt.Sprintf("REVOKE ALL ON DATABASE %s FROM PUBLIC;", quote(target.database)),
			fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s, %s;", quote(target.database), quote(target.migrator), quote(target.runtime)),
			fmt.Sprintf("REVOKE %s FROM \"bidora_local_provisioner\";", quote(target.owner)),
		}
		plan.Steps = append(plan.Steps, create, grants)
		for _, role := range []string{target.migrator, target.runtime} {
			activation := newStep(role+".activate", "postgres", "bidora_local_bootstrap", "bidora_local_bootstrap", true)
			activation.Kind = CredentialStep
			activation.CredentialRole = role
			plan.Steps = append(plan.Steps, activation)
		}
		schema := newStep(target.name+".schema", target.database, target.migrator, target.owner, true)
		schema.Statements = []string{
			`REVOKE ALL ON SCHEMA public FROM PUBLIC;`,
			fmt.Sprintf("CREATE SCHEMA bidora_fixture AUTHORIZATION %s;", quote(target.owner)),
			`REVOKE ALL ON SCHEMA bidora_fixture FROM PUBLIC;`,
			`CREATE TABLE bidora_fixture.sentinel (
    probe_key text PRIMARY KEY CHECK (octet_length(probe_key) BETWEEN 1 AND 64),
    probe_value text NOT NULL CHECK (octet_length(probe_value) <= 256)
);`,
			`REVOKE ALL ON TABLE bidora_fixture.sentinel FROM PUBLIC;`,
			fmt.Sprintf("GRANT USAGE ON SCHEMA bidora_fixture TO %s;", quote(target.runtime)),
			fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE bidora_fixture.sentinel TO %s;", quote(target.runtime)),
		}
		plan.Steps = append(plan.Steps, schema)
	}
	return plan, nil
}

func newStep(id, database, login, role string, transactional bool) Step {
	return Step{ID: id, Kind: SQLStep, Database: database, LoginRole: login,
		EffectiveRole: role, Transactional: transactional,
		StatementTimeout: 5 * time.Second, LockTimeout: time.Second}
}

type target struct{ name, database, owner, migrator, runtime string }

func targets() []target {
	var result []target
	for _, name := range []string{"control", "tenant_a", "tenant_b"} {
		database := "bidora_local_" + name
		result = append(result, target{name, database, database + "_owner", database + "_migrator", database + "_runtime"})
	}
	return result
}

// Only fixed internal identifiers reach quote; never accept arbitrary SQL names.
func quote(name string) string { return `"` + name + `"` }

func allowedCredentialRole(role string) bool {
	for _, target := range targets() {
		if role == target.migrator || role == target.runtime {
			return true
		}
	}
	return false
}
