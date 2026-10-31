// Package domain owns typed Platform integration-setting values and lifecycle invariants.
package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type Kind string
type VersionState string
type ChangeState string
type Action string

const (
	SMTP             Kind         = "SMTP"
	OpenAI           Kind         = "OPENAI"
	Draft            VersionState = "DRAFT"
	TestPassed       VersionState = "TEST_PASSED"
	TestFailed       VersionState = "TEST_FAILED"
	PendingApproval  VersionState = "PENDING_APPROVAL"
	Active           VersionState = "ACTIVE"
	Superseded       VersionState = "SUPERSEDED"
	ChangePending    ChangeState  = "PENDING_APPROVAL"
	ChangeExecuted   ChangeState  = "APPROVED_EXECUTED"
	ChangeRejected   ChangeState  = "REJECTED"
	ChangeExpired    ChangeState  = "EXPIRED"
	Activate         Action       = "ACTIVATE"
	Rollback         Action       = "ROLLBACK"
	OpenAIEndpoint                = "https://api.openai.com/v1/responses"
	ApprovalLifetime              = 30 * time.Minute
)

var ErrInvalid = errors.New("platform_setting_invalid")
var ErrConflict = errors.New("platform_setting_conflict")

type SMTPConfiguration struct {
	Host, CredentialSecretRef, TrustRootsSecretRef, SenderName, SenderAddress                  string
	ReplyToAddress                                                                             *string
	Port, ConnectTimeoutSeconds, CommandTimeoutSeconds, DeliveryTimeoutSeconds, MaxConcurrency int32
}
type OpenAIConfiguration struct {
	Disabled                                    bool `json:"disabled,omitempty"`
	Model, APIKeySecretRef, TrustRootsSecretRef string
	TimeoutSeconds, MaxConcurrency              int32
}
type Configuration struct {
	PesaPal *PesaPalConfiguration `json:"PesaPal,omitempty"`
	Kind    Kind
	SMTP    *SMTPConfiguration
	OpenAI  *OpenAIConfiguration
}
type Version struct {
	ID                        ID
	Kind                      Kind
	Sequence                  int64
	State                     VersionState
	Version                   int64
	Configuration             Configuration
	ConfigurationHash         [32]byte
	LatestTestID              ID
	CreatedBy                 ID
	CreatedAt, UpdatedAt      time.Time
	ActivatedAt, SupersededAt *time.Time
}
type TestResult struct {
	ID, VersionID, TestedBy ID
	Attempt                 int64
	Outcome, ResultCode     string
	Reason                  string
	ApplicabilityHash       [32]byte
	OccurredAt              time.Time
}
type Change struct {
	ID, TargetVersionID, TargetTestID, ExpectedActiveVersionID, MakerUserID, CheckerUserID ID
	Kind                                                                                   Kind
	Action                                                                                 Action
	State                                                                                  ChangeState
	TargetPriorState                                                                       VersionState
	Version                                                                                int64
	CreatedAt, ExpiresAt                                                                   time.Time
	DecidedAt                                                                              *time.Time
}

func NewConfiguration(kind Kind, smtp *SMTPConfiguration, openai *OpenAIConfiguration) (Configuration, [32]byte, error) {
	configuration := Configuration{Kind: kind, SMTP: smtp, OpenAI: openai}
	if !configuration.Valid() {
		return Configuration{}, [32]byte{}, ErrInvalid
	}
	canonical := struct {
		Kind   Kind
		SMTP   *SMTPConfiguration
		OpenAI *OpenAIConfiguration
	}{kind, smtp, openai}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return Configuration{}, [32]byte{}, ErrInvalid
	}
	return configuration, sha256.Sum256(encoded), nil
}
func (c Configuration) Valid() bool {
	switch c.Kind {
	case PesaPal:
		return c.PesaPal != nil && c.SMTP == nil && c.OpenAI == nil && c.PesaPal.valid()
	case SMTP:
		return c.PesaPal == nil && c.SMTP != nil && c.OpenAI == nil && c.SMTP.valid()
	case OpenAI:
		return c.PesaPal == nil && c.OpenAI != nil && c.SMTP == nil && c.OpenAI.valid()
	default:
		return false
	}
}
func (c Configuration) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED platform integration configuration]")
}
func (c SMTPConfiguration) valid() bool {
	return bounded(c.Host, 1, 253) && !strings.ContainsAny(c.Host, "/ \\\t\r\n") && c.Port >= 1 && c.Port <= 65535 && secretRef(c.CredentialSecretRef) && secretRef(c.TrustRootsSecretRef) && safeText(c.SenderName, 1, 200) && address(c.SenderAddress) && optionalAddress(c.ReplyToAddress) && between(c.ConnectTimeoutSeconds, 1, 30) && between(c.CommandTimeoutSeconds, 1, 60) && between(c.DeliveryTimeoutSeconds, 1, 120) && between(c.MaxConcurrency, 1, 100)
}
func (c OpenAIConfiguration) valid() bool {
	return safeText(c.Model, 1, 200) && secretRef(c.APIKeySecretRef) && secretRef(c.TrustRootsSecretRef) && between(c.TimeoutSeconds, 10, 900) && between(c.MaxConcurrency, 1, 100)
}
func TestApplicabilityHash(configurationHash [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("platform-integration-test:v1:"), configurationHash[:]...))
}

func NewChange(id ID, version Version, test TestResult, action Action, expectedActive, maker ID, at time.Time) (Change, error) {
	if id.IsZero() || version.ID.IsZero() || test.ID.IsZero() || test.VersionID != version.ID || test.Outcome != "PASSED" || test.ApplicabilityHash != TestApplicabilityHash(version.ConfigurationHash) || maker.IsZero() || at.IsZero() || (action != Activate && action != Rollback) {
		return Change{}, ErrInvalid
	}
	prior := version.State
	if (action == Activate && prior != TestPassed) || (action == Rollback && prior != Superseded) || version.LatestTestID != test.ID {
		return Change{}, ErrConflict
	}
	if action == Rollback && (expectedActive.IsZero() || expectedActive == version.ID) {
		return Change{}, ErrConflict
	}
	at = at.UTC()
	return Change{ID: id, TargetVersionID: version.ID, TargetTestID: test.ID, ExpectedActiveVersionID: expectedActive, MakerUserID: maker, Kind: version.Kind, Action: action, State: ChangePending, TargetPriorState: prior, Version: 1, CreatedAt: at, ExpiresAt: at.Add(ApprovalLifetime)}, nil
}
func (c Change) CanDecide(checker ID, at time.Time) error {
	if c.State != ChangePending || c.Version < 1 || checker.IsZero() || checker == c.MakerUserID || at.IsZero() {
		return ErrConflict
	}
	if !at.UTC().Before(c.ExpiresAt) {
		return ErrConflict
	}
	return nil
}
func bounded(v string, min, max int) bool {
	return utf8.ValidString(v) && len(v) >= min && len(v) <= max && strings.TrimSpace(v) == v
}
func safeText(v string, min, max int) bool {
	return bounded(v, min, max) && !strings.ContainsAny(v, "\x00\r\n")
}
func secretRef(v string) bool        { return safeText(v, 1, 500) }
func between(v, min, max int32) bool { return v >= min && v <= max }
func optionalAddress(v *string) bool { return v == nil || address(*v) }
func address(v string) bool {
	if !bounded(v, 3, 254) {
		return false
	}
	for _, r := range v {
		if r > 127 {
			return false
		}
	}
	parsed, err := mail.ParseAddress(v)
	return err == nil && parsed.Name == "" && parsed.Address == v && strings.Contains(v, "@")
}
