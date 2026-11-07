// Package application owns Platform integration-setting use-case sequencing.
package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"bidora/backend/internal/platformsettings/domain"
)

type Subject struct {
	ActorID, SessionID domain.ID
	ContextGeneration  int64
}
type Authority struct {
	UserID                          domain.ID
	Capabilities                    map[string]bool
	EffectiveOwner                  bool
	AssuranceLevel, AssuranceMethod string
	AssuredAt                       time.Time
}
type Command struct {
	ID, ActorID          domain.ID
	KeyHash, RequestHash [32]byte
	State, ResponseState string
	TargetID             domain.ID
	ResponseVersion      int64
}
type Head struct {
	Kind                     domain.Kind
	NextSequence, Generation int64
	ActiveVersionID          domain.ID
}
type TestOutcome struct {
	Passed            bool
	ResultCode        string
	ApplicabilityHash [32]byte
}
type CreateInput struct {
	ReusePesaPalCredentials          bool
	Secrets                          []SealedIntegrationSecret
	ReuseOpenAIKey, ReuseOpenAIRoots bool
	Subject                          Subject
	Configuration                    domain.Configuration
	Reason, IdempotencyKey           string
	CorrelationID                    domain.ID
}
type TestInput struct {
	Subject                Subject
	VersionID              domain.ID
	ExpectedVersion        int64
	Reason, IdempotencyKey string
	CorrelationID          domain.ID
}
type ProposeInput struct {
	Subject                Subject
	VersionID              domain.ID
	ExpectedVersion        int64
	Action                 domain.Action
	Reason, IdempotencyKey string
	CorrelationID          domain.ID
}
type DecideInput struct {
	Subject                Subject
	ChangeID               domain.ID
	ExpectedVersion        int64
	Approve                bool
	Reason, IdempotencyKey string
	CorrelationID          domain.ID
}
type Evidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TargetID domain.ID
	Action, EventType, Kind, State, ResultCode          string
	Version                                             int64
	OccurredAt                                          time.Time
}

type Tx interface {
	InsertSealedSecret(context.Context, SealedIntegrationSecret, domain.ID, time.Time) error
	Authorize(context.Context, Subject, time.Time) (Authority, error)
	GetCommand(context.Context, domain.ID, string, [32]byte) (Command, bool, error)
	LockHead(context.Context, domain.Kind) (Head, error)
	InsertVersion(context.Context, domain.Version, string) error
	CompleteCommand(context.Context, Command, string, domain.ID, time.Time) error
	LockVersion(context.Context, domain.ID) (domain.Version, bool, error)
	GetTest(context.Context, domain.ID) (domain.TestResult, bool, error)
	BeginTest(context.Context, Command, domain.ID, time.Time) error
	CommitTest(context.Context, Command, domain.Version, domain.TestResult) error
	InsertChange(context.Context, domain.Change, string, domain.Version, Command) error
	LockChange(context.Context, domain.ID) (domain.Change, bool, error)
	RejectChange(context.Context, domain.Change, domain.Version, domain.ID, string, time.Time, Command) error
	ExecuteChange(context.Context, domain.Change, domain.Version, Head, domain.ID, domain.ID, string, time.Time, Command) error
	ExpireChange(context.Context, domain.Change, domain.Version, time.Time) error
	AppendEvidence(context.Context, Evidence) error
}
type UnitOfWork interface {
	WithinSettings(context.Context, func(context.Context, Tx) error) error
}
type TestExecution struct {
	Version    domain.Version
	ResultCode string
}
type Tester interface {
	Test(context.Context, domain.Configuration, domain.ID) (TestOutcome, error)
}
type IDGenerator interface{ NewID() (domain.ID, error) }
type Clock interface{ Now() time.Time }
type Service struct {
	uow    UnitOfWork
	tester Tester
	ids    IDGenerator
	clock  Clock
}

var ErrInvalid = errors.New("platform_setting_request_invalid")
var ErrForbidden = errors.New("platform_setting_forbidden")
var ErrConflict = errors.New("platform_setting_conflict")
var ErrInProgress = errors.New("platform_setting_operation_in_progress")
var ErrUnavailable = errors.New("platform_setting_unavailable")
var ErrNotFound = errors.New("platform_setting_not_found")

func NewService(uow UnitOfWork, tester Tester, ids IDGenerator, clock Clock) (*Service, error) {
	if uow == nil || tester == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_setting_service_invalid")
	}
	return &Service{uow, tester, ids, clock}, nil
}

func (s *Service) AuthorizeCreate(ctx context.Context, subject Subject, kind domain.Kind) error {
	if ctx == nil || !validSubject(subject) || (kind != domain.SMTP && kind != domain.OpenAI && kind != domain.PesaPal) {
		return ErrInvalid
	}
	at := s.clock.Now().UTC()
	err := s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, subject, at)
		if e != nil {
			return e
		}
		if !authority.Capabilities[manageCapability(kind)] || !recent(authority, at) || kind == domain.SMTP && (!authority.EffectiveOwner || !authority.Capabilities[activateCapability(kind)]) {
			return ErrForbidden
		}
		return nil
	})
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (result domain.Version, err error) {
	if ctx == nil || !validSubject(in.Subject) || !in.Configuration.Valid() || !text(in.Reason, 1000) || !key(in.IdempotencyKey) || in.CorrelationID.IsZero() {
		return result, ErrInvalid
	}
	if in.Configuration.PesaPal != nil {
		copy := *in.Configuration.PesaPal
		in.Configuration.PesaPal = &copy
	}
	if in.Configuration.OpenAI != nil {
		copy := *in.Configuration.OpenAI
		in.Configuration.OpenAI = &copy
	}
	at := s.clock.Now().UTC()
	hash := fingerprint("create", in.Configuration, in.Reason)
	if in.ReuseOpenAIKey || in.ReuseOpenAIRoots {
		hash = fingerprint("create-openai-retain-v1", in.Configuration, in.Reason, in.ReuseOpenAIKey, in.ReuseOpenAIRoots)
	}
	if in.ReusePesaPalCredentials {
		hash = fingerprint("create-pesapal-retain-v1", in.Configuration, in.Reason, true)
	}
	keyHash := keyFingerprint("create", in.IdempotencyKey)
	err = s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, in.Subject, at)
		if e != nil {
			return e
		}
		if !authority.Capabilities[manageCapability(in.Configuration.Kind)] || !recent(authority, at) || in.Configuration.Kind == domain.SMTP && (!authority.EffectiveOwner || !authority.Capabilities[activateCapability(domain.SMTP)]) {
			return ErrForbidden
		}
		replay, found, e := tx.GetCommand(txCtx, in.Subject.ActorID, "CREATE_VERSION", keyHash)
		if e != nil {
			return e
		}
		if found {
			return replayVersion(txCtx, tx, replay, hash, &result)
		}
		head, e := tx.LockHead(txCtx, in.Configuration.Kind)
		if e != nil {
			return e
		}
		if in.Configuration.Kind == domain.OpenAI && (in.ReuseOpenAIKey || in.ReuseOpenAIRoots) {
			active, found, e := tx.LockVersion(txCtx, head.ActiveVersionID)
			if e != nil {
				return e
			}
			if !found || active.Configuration.OpenAI == nil {
				return ErrInvalid
			}
			if in.ReuseOpenAIKey {
				in.Configuration.OpenAI.APIKeySecretRef = active.Configuration.OpenAI.APIKeySecretRef
			}
			if in.ReuseOpenAIRoots {
				in.Configuration.OpenAI.TrustRootsSecretRef = active.Configuration.OpenAI.TrustRootsSecretRef
			}
		}
		if in.Configuration.Kind == domain.PesaPal && in.ReusePesaPalCredentials {
			active, found, e := tx.LockVersion(txCtx, head.ActiveVersionID)
			if e != nil {
				return e
			}
			if !found || active.Configuration.PesaPal == nil {
				return ErrInvalid
			}
			in.Configuration.PesaPal.CredentialsSecretRef = active.Configuration.PesaPal.CredentialsSecretRef
		}
		for _, secret := range in.Secrets {
			if e := tx.InsertSealedSecret(txCtx, secret, in.Subject.ActorID, at); e != nil {
				return e
			}
		}
		ids, e := newIDs(s.ids, 4)
		if e != nil {
			return e
		}
		configurationHash, e := in.Configuration.Hash()
		if e != nil {
			return ErrInvalid
		}
		result = domain.Version{ID: ids[1], Kind: in.Configuration.Kind, Sequence: head.NextSequence, State: domain.Draft, Version: 1, Configuration: in.Configuration, ConfigurationHash: configurationHash, CreatedBy: in.Subject.ActorID, CreatedAt: at, UpdatedAt: at}
		if e = tx.InsertVersion(txCtx, result, in.Reason); e != nil {
			return e
		}
		if result.Kind == domain.SMTP {
			publisher, ok := tx.(DirectSMTPPublisher)
			if !ok {
				return ErrUnavailable
			}
			history, e := s.ids.NewID()
			if e != nil || history.IsZero() {
				return ErrUnavailable
			}
			if e = publisher.PublishSMTP(txCtx, result, head, in.Subject.ActorID, history, at); e != nil {
				return e
			}
			result.State = domain.Active
			result.Version++
			result.ActivatedAt = &at
		}
		command := Command{ID: ids[0], ActorID: in.Subject.ActorID, KeyHash: keyHash, RequestHash: hash, State: "COMPLETED", ResponseState: string(result.State), TargetID: result.ID, ResponseVersion: result.Version}
		if e = tx.CompleteCommand(txCtx, command, "CREATE_VERSION", in.Subject.ActorID, at); e != nil {
			return e
		}
		return tx.AppendEvidence(txCtx, Evidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, ActorID: in.Subject.ActorID, TargetID: result.ID, Action: "PlatformIntegrationVersionCreated", EventType: "platform.integration.version.created.v1", Kind: string(result.Kind), State: string(result.State), Version: 1, OccurredAt: at})
	})
	if err != nil {
		return domain.Version{}, mapError(err)
	}
	return result, nil
}

func (s *Service) Test(ctx context.Context, in TestInput) (result TestExecution, err error) {
	if ctx == nil || !validSubject(in.Subject) || in.VersionID.IsZero() || in.ExpectedVersion < 1 || !text(in.Reason, 1000) || !key(in.IdempotencyKey) || in.CorrelationID.IsZero() {
		return result, ErrInvalid
	}
	at := s.clock.Now().UTC()
	requestHash := fingerprint("test", in.VersionID, in.ExpectedVersion, in.Reason)
	keyHash := keyFingerprint("test", in.IdempotencyKey)
	var prepared domain.Version
	var command Command
	replayed := false
	err = s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, in.Subject, at)
		if e != nil {
			return e
		}
		prepared, _, e = tx.LockVersion(txCtx, in.VersionID)
		if e != nil {
			return e
		}
		if prepared.ID.IsZero() {
			return ErrConflict
		}
		if !authority.Capabilities[testCapability(prepared.Kind)] || !recent(authority, at) {
			return ErrForbidden
		}
		existing, found, e := tx.GetCommand(txCtx, in.Subject.ActorID, "TEST_VERSION", keyHash)
		if e != nil {
			return e
		}
		if found {
			if existing.RequestHash != requestHash {
				return ErrConflict
			}
			if existing.State == "PENDING" {
				return ErrInProgress
			}
			if e = replayVersion(txCtx, tx, existing, requestHash, &result.Version); e != nil {
				return e
			}
			test, ok, e := tx.GetTest(txCtx, result.Version.LatestTestID)
			if e != nil || !ok {
				return ErrConflict
			}
			result.ResultCode = test.ResultCode
			replayed = true
			return nil
		}
		if prepared.Version != in.ExpectedVersion || !(testable(prepared.State) || prepared.Kind == domain.SMTP && prepared.State == domain.Active) {
			return ErrConflict
		}
		id, e := s.ids.NewID()
		if e != nil || id.IsZero() {
			return ErrUnavailable
		}
		command = Command{ID: id, ActorID: in.Subject.ActorID, KeyHash: keyHash, RequestHash: requestHash, State: "PENDING", TargetID: prepared.ID}
		return tx.BeginTest(txCtx, command, prepared.ID, at)
	})
	if err != nil {
		return result, mapError(err)
	}
	if replayed {
		return result, nil
	}
	outcome, testErr := s.tester.Test(ctx, prepared.Configuration, in.Subject.ActorID)
	if testErr != nil {
		outcome = TestOutcome{Passed: false, ResultCode: "CONNECTIVITY_FAILED"}
	}
	outcome.ApplicabilityHash = domain.TestApplicabilityHash(prepared.ConfigurationHash)
	if !validOutcome(prepared.Kind, outcome) {
		outcome = TestOutcome{Passed: false, ResultCode: "INVALID_RESPONSE", ApplicabilityHash: domain.TestApplicabilityHash(prepared.ConfigurationHash)}
	}
	finished := s.clock.Now().UTC()
	err = s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, in.Subject, finished)
		if e != nil {
			return e
		}
		current, found, e := tx.LockVersion(txCtx, in.VersionID)
		if e != nil {
			return e
		}
		if !found || current.Version != in.ExpectedVersion || current.ConfigurationHash != prepared.ConfigurationHash || !authority.Capabilities[testCapability(current.Kind)] || !recent(authority, finished) {
			return ErrConflict
		}
		ids, e := newIDs(s.ids, 3)
		if e != nil {
			return e
		}
		state := domain.TestFailed
		outcomeText := "FAILED"
		if outcome.Passed {
			state = domain.TestPassed
			outcomeText = "PASSED"
		}
		test := domain.TestResult{ID: ids[0], VersionID: current.ID, TestedBy: in.Subject.ActorID, Outcome: outcomeText, ResultCode: outcome.ResultCode, Reason: in.Reason, ApplicabilityHash: outcome.ApplicabilityHash, OccurredAt: finished}
		if current.Kind == domain.SMTP && current.State == domain.Active {
			state = domain.Active
		}
		current.State = state
		current.Version++
		current.LatestTestID = test.ID
		current.UpdatedAt = finished
		command.ResponseState = string(current.State)
		command.ResponseVersion = current.Version
		command.State = "COMPLETED"
		if e = tx.CommitTest(txCtx, command, current, test); e != nil {
			return e
		}
		if e = tx.AppendEvidence(txCtx, Evidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: in.CorrelationID, ActorID: in.Subject.ActorID, TargetID: current.ID, Action: "PlatformIntegrationVersionTested", EventType: "platform.integration.version.tested.v1", Kind: string(current.Kind), State: string(current.State), ResultCode: outcome.ResultCode, Version: current.Version, OccurredAt: finished}); e != nil {
			return e
		}
		result = TestExecution{Version: current, ResultCode: outcome.ResultCode}
		return nil
	})
	if err != nil {
		return TestExecution{}, mapError(err)
	}
	return result, nil
}

func (s *Service) Propose(ctx context.Context, in ProposeInput) (result domain.Change, err error) {
	if ctx == nil || !validSubject(in.Subject) || in.VersionID.IsZero() || in.ExpectedVersion < 1 || !text(in.Reason, 1000) || !key(in.IdempotencyKey) || in.CorrelationID.IsZero() {
		return result, ErrInvalid
	}
	at := s.clock.Now().UTC()
	requestHash := fingerprint("propose", in.VersionID, in.ExpectedVersion, in.Action, in.Reason)
	keyHash := keyFingerprint("propose", in.IdempotencyKey)
	err = s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, in.Subject, at)
		if e != nil {
			return e
		}
		version, found, e := tx.LockVersion(txCtx, in.VersionID)
		if e != nil {
			return e
		}
		if !found {
			return ErrConflict
		}
		needed := activateCapability(version.Kind)
		if in.Action == domain.Rollback && (!authority.Capabilities["platform.integration.rollback"] || !authority.Capabilities[needed]) {
			return ErrForbidden
		}
		if in.Action == domain.Activate && !authority.Capabilities[needed] {
			return ErrForbidden
		}
		if !recent(authority, at) {
			return ErrForbidden
		}
		replay, exists, e := tx.GetCommand(txCtx, in.Subject.ActorID, "PROPOSE_ACTIVATION", keyHash)
		if e != nil {
			return e
		}
		if exists {
			if replay.RequestHash != requestHash {
				return ErrConflict
			}
			existing, ok, e := tx.LockChange(txCtx, replay.TargetID)
			if e != nil || !ok {
				return ErrConflict
			}
			existing.State = domain.ChangeState(replay.ResponseState)
			existing.Version = replay.ResponseVersion
			result = existing
			return nil
		}
		if version.Version != in.ExpectedVersion {
			return ErrConflict
		}
		if version.Kind == domain.SMTP {
			return ErrInvalid
		} // SMTP saves apply directly.
		test, ok, e := tx.GetTest(txCtx, version.LatestTestID)
		if e != nil {
			return e
		}
		if !ok {
			return ErrConflict
		}
		head, e := tx.LockHead(txCtx, version.Kind)
		if e != nil {
			return e
		}
		ids, e := newIDs(s.ids, 4)
		if e != nil {
			return e
		}
		result, e = domain.NewChange(ids[1], version, test, in.Action, head.ActiveVersionID, in.Subject.ActorID, at)
		if e != nil {
			return ErrConflict
		}
		command := Command{ID: ids[0], ActorID: in.Subject.ActorID, KeyHash: keyHash, RequestHash: requestHash, State: "COMPLETED", ResponseState: string(domain.ChangePending), TargetID: result.ID, ResponseVersion: 1}
		if e = tx.InsertChange(txCtx, result, in.Reason, version, command); e != nil {
			return e
		}
		return tx.AppendEvidence(txCtx, Evidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, ActorID: in.Subject.ActorID, TargetID: result.ID, Action: "PlatformIntegrationActivationProposed", EventType: "platform.integration.activation.proposed.v1", Kind: string(result.Kind), State: string(result.State), Version: 1, OccurredAt: at})
	})
	if err != nil {
		return domain.Change{}, mapError(err)
	}
	return result, nil
}

func (s *Service) Decide(ctx context.Context, in DecideInput) (result domain.Change, err error) {
	if ctx == nil || !validSubject(in.Subject) || in.ChangeID.IsZero() || in.ExpectedVersion < 1 || !text(in.Reason, 1000) || !key(in.IdempotencyKey) || in.CorrelationID.IsZero() {
		return result, ErrInvalid
	}
	at := s.clock.Now().UTC()
	operation := "REJECT_CHANGE"
	if in.Approve {
		operation = "APPROVE_CHANGE"
	}
	requestHash := fingerprint(operation, in.ChangeID, in.ExpectedVersion, in.Reason)
	keyHash := keyFingerprint(operation, in.IdempotencyKey)
	expired := false
	err = s.uow.WithinSettings(ctx, func(txCtx context.Context, tx Tx) error {
		authority, e := tx.Authorize(txCtx, in.Subject, at)
		if e != nil {
			return e
		}
		if !authority.EffectiveOwner || !authority.Capabilities["platform.governance.approve"] || !recent(authority, at) {
			return ErrForbidden
		}
		replay, exists, e := tx.GetCommand(txCtx, in.Subject.ActorID, operation, keyHash)
		if e != nil {
			return e
		}
		if exists {
			if replay.RequestHash != requestHash {
				return ErrConflict
			}
			existing, ok, e := tx.LockChange(txCtx, replay.TargetID)
			if e != nil || !ok {
				return ErrConflict
			}
			existing.State = domain.ChangeState(replay.ResponseState)
			existing.Version = replay.ResponseVersion
			result = existing
			return nil
		}
		change, found, e := tx.LockChange(txCtx, in.ChangeID)
		if e != nil {
			return e
		}
		if !found || change.Kind == domain.SMTP || change.Version != in.ExpectedVersion || change.MakerUserID == in.Subject.ActorID {
			return ErrConflict
		}
		version, found, e := tx.LockVersion(txCtx, change.TargetVersionID)
		if e != nil || !found {
			return ErrConflict
		}
		if !at.Before(change.ExpiresAt) {
			if e = tx.ExpireChange(txCtx, change, version, at); e != nil {
				return e
			}
			expired = true
			return nil
		}
		head, e := tx.LockHead(txCtx, change.Kind)
		if e != nil {
			return e
		}
		ids, e := newIDs(s.ids, 4)
		if e != nil {
			return e
		}
		responseState := domain.ChangeRejected
		if in.Approve {
			responseState = domain.ChangeExecuted
		}
		command := Command{ID: ids[0], ActorID: in.Subject.ActorID, KeyHash: keyHash, RequestHash: requestHash, State: "COMPLETED", ResponseState: string(responseState), TargetID: change.ID, ResponseVersion: change.Version + 1}
		if in.Approve {
			if version.LatestTestID != change.TargetTestID || head.ActiveVersionID != change.ExpectedActiveVersionID {
				return ErrConflict
			}
			historyID := ids[1]
			if e = tx.ExecuteChange(txCtx, change, version, head, in.Subject.ActorID, historyID, in.Reason, at, command); e != nil {
				return e
			}
			change.State = domain.ChangeExecuted
		} else {
			if e = tx.RejectChange(txCtx, change, version, in.Subject.ActorID, in.Reason, at, command); e != nil {
				return e
			}
			change.State = domain.ChangeRejected
		}
		change.CheckerUserID = in.Subject.ActorID
		change.Version++
		change.DecidedAt = &at
		result = change
		return tx.AppendEvidence(txCtx, Evidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, ActorID: in.Subject.ActorID, TargetID: change.ID, Action: map[bool]string{true: "PlatformIntegrationActivationApproved", false: "PlatformIntegrationActivationRejected"}[in.Approve], EventType: map[bool]string{true: "platform.integration.activation.executed.v1", false: "platform.integration.activation.rejected.v1"}[in.Approve], Kind: string(change.Kind), State: string(change.State), Version: change.Version, OccurredAt: at})
	})
	if err != nil {
		return domain.Change{}, mapError(err)
	}
	if expired {
		return domain.Change{}, ErrConflict
	}
	return result, nil
}
func replayVersion(ctx context.Context, tx Tx, c Command, hash [32]byte, out *domain.Version) error {
	if c.RequestHash != hash {
		return ErrConflict
	}
	if c.State == "PENDING" {
		return ErrInProgress
	}
	v, found, e := tx.LockVersion(ctx, c.TargetID)
	if e != nil {
		return e
	}
	if !found {
		return ErrConflict
	}
	v.State = domain.VersionState(c.ResponseState)
	v.Version = c.ResponseVersion
	*out = v
	return nil
}
func manageCapability(k domain.Kind) string {
	if k == domain.PesaPal {
		return "platform.integration.pesapal.manage"
	}
	if k == domain.SMTP {
		return "platform.integration.smtp.manage"
	}
	return "platform.integration.ai.manage"
}
func testCapability(k domain.Kind) string {
	if k == domain.PesaPal {
		return "platform.integration.pesapal.test"
	}
	if k == domain.SMTP {
		return "platform.integration.smtp.test"
	}
	return "platform.integration.ai.test"
}
func activateCapability(k domain.Kind) string {
	if k == domain.PesaPal {
		return "platform.integration.pesapal.activate"
	}
	if k == domain.SMTP {
		return "platform.integration.smtp.activate"
	}
	return "platform.integration.ai.activate"
}
func validSubject(v Subject) bool {
	return !v.ActorID.IsZero() && !v.SessionID.IsZero() && v.ContextGeneration > 0
}
func currentAAL2(a Authority, at time.Time) bool {
	return platformpolicy.AdministratorAssurance(a.AssuranceLevel, a.AssuranceMethod) && !a.AssuredAt.IsZero() && !a.AssuredAt.After(at)
}
func recent(a Authority, at time.Time) bool {
	return currentAAL2(a, at) && at.Sub(a.AssuredAt) <= 15*time.Minute
}
func text(v string, max int) bool {
	return utf8.ValidString(v) && len(v) <= max && strings.TrimSpace(v) != ""
}
func key(v string) bool {
	return utf8.ValidString(v) && len(v) >= 1 && len(v) <= 128 && strings.TrimSpace(v) == v
}
func fingerprint(purpose string, values ...any) [32]byte {
	encoded, _ := json.Marshal(values)
	return sha256.Sum256(append([]byte("platform-settings:"+purpose+":v1:"), encoded...))
}
func keyFingerprint(purpose, key string) [32]byte {
	return sha256.Sum256([]byte("platform-settings:" + purpose + ":v1:" + key))
}
func newIDs(g IDGenerator, n int) ([]domain.ID, error) {
	out := make([]domain.ID, n)
	for i := range out {
		id, e := g.NewID()
		if e != nil || id.IsZero() {
			return nil, ErrUnavailable
		}
		out[i] = id
	}
	return out, nil
}
func testable(state domain.VersionState) bool {
	return state == domain.Draft || state == domain.TestPassed || state == domain.TestFailed || state == domain.Superseded
}
func validOutcome(k domain.Kind, v TestOutcome) bool {
	if v.ApplicabilityHash == [32]byte{} {
		return false
	}
	if v.Passed {
		return (k == domain.PesaPal && (v.ResultCode == "PESAPAL_AUTHENTICATION_VERIFIED" || v.ResultCode == "PESAPAL_DISABLED_CONFIG_VERIFIED")) || (k == domain.SMTP && v.ResultCode == "SMTP_STARTTLS_VERIFIED") || (k == domain.OpenAI && (v.ResultCode == "OPENAI_STRUCTURED_RESPONSE_VERIFIED" || v.ResultCode == "OPENAI_DISABLED_CONFIG_VERIFIED"))
	}
	switch v.ResultCode {
	case "CONNECTIVITY_FAILED", "AUTHENTICATION_FAILED", "TLS_FAILED", "PROVIDER_REJECTED", "INVALID_RESPONSE":
		return true
	}
	return false
}
func mapError(err error) error {
	if errors.Is(err, ErrInvalid) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrConflict) || errors.Is(err, ErrInProgress) || errors.Is(err, ErrNotFound) {
		return err
	}
	return ErrUnavailable
}
