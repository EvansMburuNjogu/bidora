package platformsettings_test

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/openairuntime"
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type aiVersions struct{ version domain.Version }

func (v *aiVersions) ActiveOpenAI(context.Context) (domain.Version, error) { return v.version, nil }

type aiPolicy struct{ value app.OperationalPolicy }

func (p *aiPolicy) CurrentPolicy(context.Context) (app.OperationalPolicy, error) { return p.value, nil }

type aiSecrets struct {
	roots []byte
	fail  bool
	refs  []string
}

func (s *aiSecrets) WithSecret(_ context.Context, ref string, use func([]byte) error) error {
	s.refs = append(s.refs, ref)
	if s.fail {
		return errors.New("synthetic unavailable")
	}
	if ref == "managed-key" {
		return use([]byte("synthetic-key"))
	}
	return use(s.roots)
}
func TestManagedOpenAIUsesFreshModelTimeoutConcurrencyFlagsAndSecretReferences(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("no provider request authorized") }))
	defer server.Close()
	cert := server.Certificate()
	secrets := &aiSecrets{roots: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})}
	cfg, hash, e := domain.NewConfiguration(domain.OpenAI, nil, &domain.OpenAIConfiguration{Model: "gpt-4.1-mini", APIKeySecretRef: "managed-key", TrustRootsSecretRef: "managed-roots", TimeoutSeconds: 73, MaxConcurrency: 1})
	if e != nil {
		t.Fatal(e)
	}
	versions := &aiVersions{domain.Version{ID: domain.ID{1}, Kind: domain.OpenAI, State: domain.Active, Version: 1, Configuration: cfg, ConfigurationHash: hash}}
	policies := &aiPolicy{validPolicy()}
	runtime, e := openairuntime.New(versions, secrets, policies)
	if e != nil {
		t.Fatal(e)
	}
	resolved, release, e := runtime.Resolve(context.Background())
	if e != nil || resolved.Model != "gpt-4.1-mini" || resolved.Timeout.Seconds() != 73 || resolved.APIKey != "synthetic-key" || len(resolved.RootCAs.Subjects()) != 1 {
		t.Fatal("managed parameters dropped", e)
	}
	if len(secrets.refs) != 2 || secrets.refs[0] != "managed-key" || secrets.refs[1] != "managed-roots" {
		t.Fatal("wrong credentials source")
	}
	if _, _, e = runtime.Resolve(context.Background()); e == nil {
		t.Fatal("concurrency ignored")
	}
	release()
	release() // a lease releases only once
	versions.version.Configuration.OpenAI.Model = "gpt-4.1"
	versions.version.Configuration.OpenAI.TimeoutSeconds = 91
	resolved, release, e = runtime.Resolve(context.Background())
	if e != nil || resolved.Model != "gpt-4.1" || resolved.Timeout.Seconds() != 91 {
		t.Fatal("stale configuration used", e)
	}
	release()
	policies.value.AIEnabled = false
	if _, _, e = runtime.Resolve(context.Background()); e == nil {
		t.Fatal("global AI disable ignored")
	}
	policies.value.AIEnabled = true
	versions.version.Configuration.OpenAI.Disabled = true
	if _, _, e = runtime.Resolve(context.Background()); e == nil {
		t.Fatal("adapter disable ignored")
	}
	versions.version.Configuration.OpenAI.Disabled = false
	secrets.fail = true
	if _, _, e = runtime.Resolve(context.Background()); e == nil {
		t.Fatal("missing credential accepted")
	}
	secrets.fail = false
	_, release, e = runtime.Resolve(context.Background())
	if e != nil {
		t.Fatal("failed resolve leaked concurrency permit")
	}
	release()
	policies.value.MaintenanceEnabled = true
	policies.value.MaintenanceMessage = "Maintenance"
	if _, _, e = runtime.Resolve(context.Background()); e == nil {
		t.Fatal("maintenance ignored")
	}
}
