package integration_test

import (
	"context"
	"crypto/x509"
	"errors"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	oppapp "bidora/backend/internal/opportunity/application"
	opppersistence "bidora/backend/internal/opportunity/persistence"
	oppsql "bidora/backend/internal/opportunity/persistence/sqlc"
	opptransport "bidora/backend/internal/opportunity/transport"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type liveAuthorizer struct{ authority oppapp.CaptureAuthority }

func (a liveAuthorizer) AuthorizeCapture(context.Context, oppapp.CaptureSubject) (oppapp.CaptureAuthority, error) {
	return a.authority, nil
}

type liveDocuments struct{}

func (liveDocuments) CheckCaptureSource(context.Context, oppapp.CaptureAuthority, string) error {
	return nil
}

type liveIDs struct {
	values []oppapp.ID
	index  int
}

func (g *liveIDs) NewID() (oppapp.ID, error) {
	if g.index >= len(g.values) {
		return oppapp.ID{}, errors.New("integration id exhausted")
	}
	id := g.values[g.index]
	g.index++
	return id, nil
}

type liveClock struct{ at time.Time }

func (c liveClock) Now() time.Time    { return c.at }
func liveID() oppapp.ID               { return oppapp.ID(uuid.New()) }
func liveUUID(id oppapp.ID) uuid.UUID { return uuid.UUID(id) }
func idBatch() []oppapp.ID            { return []oppapp.ID{liveID(), liveID(), liveID(), liveID(), liveID()} }

func livePostgresConfig(t *testing.T, envName string) postgres.Config {
	t.Helper()
	raw := os.Getenv(envName)
	caPath := os.Getenv("BIDORA_TEST_POSTGRES_CA_FILE")
	if raw == "" || caPath == "" {
		t.Skip("live PostgreSQL integration environment is not configured")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid integration endpoint")
	}
	password, ok := parsed.User.Password()
	if !ok {
		t.Fatal("integration endpoint password missing")
	}
	portValue, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatal("integration endpoint port invalid")
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatal("integration CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("integration CA invalid")
	}
	return postgres.Config{Host: parsed.Hostname(), Port: uint16(portValue), Database: parsed.Path[1:], User: parsed.User.Username(), Password: password, RootCAs: roots, MaxConnections: 4, ConnectTimeout: 3 * time.Second, OperationTimeout: 5 * time.Second, LockTimeout: time.Second, MaxLifetime: time.Minute, MaxIdleTime: 30 * time.Second, HealthCheckPeriod: 10 * time.Second}
}

func livePool(t *testing.T, envName string) *postgres.Pool {
	t.Helper()
	pool, err := postgres.New(livePostgresConfig(t, envName))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pool.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return pool
}
func liveCommand(t *testing.T, title string) oppapp.CaptureCommand {
	t.Helper()
	command, err := opptransport.DecodeCaptureJSON([]byte(`{"title":"` + title + `","buyer_name":null}`))
	if err != nil {
		t.Fatal(err)
	}
	return command
}
func executeLive(t *testing.T, pool *postgres.Pool, tenant, actor oppapp.ID, generation int64, key string, command oppapp.CaptureCommand, ids []oppapp.ID, at time.Time) (oppapp.CaptureResult, error) {
	t.Helper()
	store, err := opppersistence.NewCaptureStore(pool, tenant, 1)
	if err != nil {
		t.Fatal(err)
	}
	executor := oppapp.CaptureExecutor{Authorizer: liveAuthorizer{oppapp.CaptureAuthority{ActorID: actor, TenantID: tenant, AssignmentGeneration: generation}}, Documents: liveDocuments{}, UOW: store, IDs: &liveIDs{values: ids}, Clock: liveClock{at}}
	return executor.Execute(context.Background(), oppapp.ExecuteCaptureInput{Subject: oppapp.CaptureSubject{IdentityID: actor, SelectedTenantID: tenant, ContextGeneration: 1}, IdempotencyKey: key, CorrelationID: liveID(), Command: command})
}
func rowCount(t *testing.T, pool *postgres.Pool, query string, args ...any) int64 {
	t.Helper()
	var count int64
	err := pool.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, query, args...).Scan(&count)
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func TestLiveCaptureTransactionReplayRollbackAndTenantIsolation(t *testing.T) {
	tenantA := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	tenantAObserver := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tenantA.WithinReadOnlyTransaction(ctx, func(ctx context.Context, db postgres.DBTX) error {
		return opppersistence.VerifyCaptureCompatibility(ctx, oppsql.New(db))
	}); err != nil {
		t.Fatal(err)
	}
	if err := tenantB.WithinReadOnlyTransaction(ctx, func(ctx context.Context, db postgres.DBTX) error {
		return opppersistence.VerifyCaptureCompatibility(ctx, oppsql.New(db))
	}); err != nil {
		t.Fatal(err)
	}
	tenantID, actor := liveID(), liveID()
	key := uuid.NewString()
	at := time.Now().UTC().Truncate(time.Microsecond)
	firstIDs := idBatch()
	command := liveCommand(t, "Synthetic live capture "+uuid.NewString())
	result, err := executeLive(t, tenantA, tenantID, actor, 1, key, command, firstIDs, at)
	if err != nil {
		t.Fatal(err)
	}
	if result.OpportunityID != firstIDs[1] || result.SourceVersionID != firstIDs[2] || result.Replayed {
		t.Fatal("live capture result drifted")
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(firstIDs[1])) != 1 || rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.source_versions WHERE id=$1", liveUUID(firstIDs[2])) != 1 || rowCount(t, tenantAObserver, "SELECT count(*) FROM bidora_audit.audit_events WHERE target_id=$1", liveUUID(firstIDs[1])) != 1 || rowCount(t, tenantA, "SELECT count(*) FROM bidora_eventing.outbox_events WHERE aggregate_id=$1", liveUUID(firstIDs[1])) != 1 {
		t.Fatal("live capture atomic facts missing")
	}
	if rowCount(t, tenantB, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(firstIDs[1])) != 0 {
		t.Fatal("tenant B observed tenant A capture")
	}
	replay, err := executeLive(t, tenantA, tenantID, actor, 1, key, command, idBatch(), at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.OpportunityID != result.OpportunityID || !replay.CreatedAt.Equal(result.CreatedAt) {
		t.Fatal("live replay changed committed result")
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(firstIDs[1])) != 1 {
		t.Fatal("live replay duplicated state")
	}
	if _, err = executeLive(t, tenantA, tenantID, actor, 1, key, liveCommand(t, "Different synthetic input"), idBatch(), at.Add(2*time.Second)); err == nil || err.Error() != "capture_idempotency_conflict" {
		t.Fatal("live idempotency conflict not enforced", err)
	}
	rollbackIDs := idBatch()
	rollbackIDs[3] = firstIDs[3]
	rollbackKey := uuid.NewString()
	if _, err = executeLive(t, tenantA, tenantID, actor, 1, rollbackKey, liveCommand(t, "Synthetic rollback "+uuid.NewString()), rollbackIDs, at.Add(3*time.Second)); err == nil || err.Error() != "capture_persistence_failed" {
		t.Fatal("live audit failure did not abort", err)
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(rollbackIDs[1])) != 0 || rowCount(t, tenantA, "SELECT count(*) FROM bidora_commands.idempotency_records WHERE id=$1", liveUUID(rollbackIDs[0])) != 0 || rowCount(t, tenantA, "SELECT count(*) FROM bidora_eventing.outbox_events WHERE id=$1", liveUUID(rollbackIDs[4])) != 0 {
		t.Fatal("live failed transaction left partial state")
	}
	outboxRollbackIDs := idBatch()
	outboxRollbackIDs[4] = firstIDs[4]
	if _, err = executeLive(t, tenantA, tenantID, actor, 1, uuid.NewString(), liveCommand(t, "Synthetic outbox rollback "+uuid.NewString()), outboxRollbackIDs, at.Add(4*time.Second)); err == nil || err.Error() != "capture_persistence_failed" {
		t.Fatal("live outbox failure did not abort", err)
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(outboxRollbackIDs[1])) != 0 || rowCount(t, tenantA, "SELECT count(*) FROM bidora_commands.idempotency_records WHERE id=$1", liveUUID(outboxRollbackIDs[0])) != 0 || rowCount(t, tenantAObserver, "SELECT count(*) FROM bidora_audit.audit_events WHERE id=$1", liveUUID(outboxRollbackIDs[3])) != 0 {
		t.Fatal("live outbox failure left partial state")
	}
	concurrentKey := uuid.NewString()
	concurrentCommand := liveCommand(t, "Synthetic concurrent capture "+uuid.NewString())
	type concurrentOutcome struct {
		result oppapp.CaptureResult
		err    error
	}
	outcomes := make(chan concurrentOutcome, 2)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	for i := 0; i < 2; i++ {
		ids := idBatch()
		go func(ids []oppapp.ID) {
			ready.Done()
			<-start
			result, err := executeLive(t, tenantA, tenantID, actor, 1, concurrentKey, concurrentCommand, ids, at.Add(4*time.Second))
			outcomes <- concurrentOutcome{result, err}
		}(ids)
	}
	ready.Wait()
	close(start)
	firstConcurrent, secondConcurrent := <-outcomes, <-outcomes
	if firstConcurrent.err != nil || secondConcurrent.err != nil || firstConcurrent.result.OpportunityID != secondConcurrent.result.OpportunityID || firstConcurrent.result.Replayed == secondConcurrent.result.Replayed {
		t.Fatal("same-key concurrency did not converge", firstConcurrent.err, secondConcurrent.err)
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_opportunity.opportunities WHERE id=$1", liveUUID(firstConcurrent.result.OpportunityID)) != 1 {
		t.Fatal("same-key concurrency created duplicate state")
	}
	leaseIDs := idBatch()
	if _, err = executeLive(t, tenantA, tenantID, actor, 2, uuid.NewString(), liveCommand(t, "Synthetic stale lease"), leaseIDs, at.Add(4*time.Second)); err == nil || err.Error() != "capture_tenant_lease_mismatch" {
		t.Fatal("stale assignment generation accepted", err)
	}
	if rowCount(t, tenantA, "SELECT count(*) FROM bidora_commands.idempotency_records WHERE id=$1", liveUUID(leaseIDs[0])) != 0 {
		t.Fatal("stale lease reached persistence")
	}
}
