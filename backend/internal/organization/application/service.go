// Package application owns tenant-private organization profile and letterhead use cases.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var ErrNotFound = errors.New("organization_not_found")
var ErrConflict = errors.New("organization_conflict")

type Subject struct {
	IdentityID, TenantID ID
	ContextGeneration    int64
}
type Authority struct {
	ActorID, TenantID    ID
	AssignmentGeneration int64
}
type Authorizer interface {
	Authorize(context.Context, Subject, string) (Authority, error)
}
type IDGenerator interface{ NewID() (ID, error) }
type Clock interface{ Now() time.Time }
type CompanyProfileFields struct {
	RegisteredName                                                                                                    string
	TradingName                                                                                                       *string
	BusinessType, PrimaryIndustry, Description, ServicesCapabilities, WorkEmail, WorkPhone, PreferredLanguage, Locale string
	Website, PostalAddress, PhysicalAddress, RegistrationIdentifier, TaxIdentifier                                    *string
}
type CompanyProfile struct {
	ID ID
	CompanyProfileFields
	Version              int64
	CreatedBy, UpdatedBy ID
	CreatedAt, UpdatedAt time.Time
}
type SaveProfileInput struct {
	Subject         Subject
	CorrelationID   ID
	IdempotencyKey  string
	ExpectedVersion int64
	Fields          CompanyProfileFields
}
type SaveProfileCommit struct {
	CommandID, ProfileID, AuditID, OutboxID, CorrelationID ID
	Actor                                                  Authority
	ExpectedVersion                                        int64
	Fields                                                 CompanyProfileFields
	KeyHash, RequestHash                                   [32]byte
	At                                                     time.Time
}
type ProfileResult struct {
	Profile CompanyProfile
	Replay  bool
}
type LetterheadFields struct {
	RegisteredName                                                                     string
	TradingName, RegistrationIdentifier, TaxIdentifier, PostalAddress, PhysicalAddress *string
	Phone, Email                                                                       string
	Website, Slogan                                                                    *string
	HeaderLayout, FooterLayout, PageNumberStyle, PrimaryColor, SecondaryColor          string
	Disclaimer                                                                         *string
	TemplateKey                                                                        string
}
type LetterheadVersion struct {
	ID            ID
	VersionNumber int64
	LetterheadFields
	ContentHash [32]byte
	CreatedBy   ID
	CreatedAt   time.Time
}
type LetterheadConfiguration struct {
	ActiveVersionID            *ID
	NextVersionNumber, Version int64
	UpdatedBy                  *ID
	UpdatedAt                  time.Time
}
type CreateLetterheadInput struct {
	Subject                      Subject
	CorrelationID                ID
	IdempotencyKey               string
	ExpectedConfigurationVersion int64
	Fields                       LetterheadFields
}
type CreateLetterheadCommit struct {
	CommandID, VersionID, AuditID, OutboxID, CorrelationID ID
	Actor                                                  Authority
	ExpectedConfigurationVersion                           int64
	Fields                                                 LetterheadFields
	ContentHash, KeyHash, RequestHash                      [32]byte
	At                                                     time.Time
}
type LetterheadResult struct {
	Version              LetterheadVersion
	ConfigurationVersion int64
	Replay               bool
}
type ActivateLetterheadInput struct {
	Subject                      Subject
	VersionID, CorrelationID     ID
	IdempotencyKey, Reason       string
	ExpectedConfigurationVersion int64
}
type ActivateLetterheadCommit struct {
	CommandID, ActivationID, AuditID, OutboxID, VersionID, CorrelationID ID
	Actor                                                                Authority
	ExpectedConfigurationVersion                                         int64
	Reason                                                               string
	KeyHash, RequestHash                                                 [32]byte
	At                                                                   time.Time
}
type ActivationResult struct {
	Version              LetterheadVersion
	ConfigurationVersion int64
	Replay               bool
}
type Store interface {
	SaveProfile(context.Context, SaveProfileCommit) (ProfileResult, error)
	CreateLetterhead(context.Context, CreateLetterheadCommit) (LetterheadResult, error)
	ActivateLetterhead(context.Context, ActivateLetterheadCommit) (ActivationResult, error)
	GetProfile(context.Context, Authority) (CompanyProfile, error)
	GetLetterhead(context.Context, Authority, ID) (LetterheadVersion, error)
	GetActiveLetterhead(context.Context, Authority) (LetterheadVersion, LetterheadConfiguration, error)
}
type Service struct {
	auth  Authorizer
	store Store
	ids   IDGenerator
	clock Clock
}

func NewService(auth Authorizer, store Store, ids IDGenerator, clock Clock) (*Service, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("organization_service_invalid")
	}
	return &Service{auth, store, ids, clock}, nil
}
func (s *Service) SaveProfile(ctx context.Context, in SaveProfileInput) (ProfileResult, error) {
	if ctx == nil || !validSubject(in.Subject) || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || in.ExpectedVersion < 0 || !validProfile(in.Fields) {
		return ProfileResult{}, errors.New("organization_profile_invalid")
	}
	a, err := s.auth.Authorize(ctx, in.Subject, "tenderpreneur.organization.profile.manage")
	if err != nil {
		return ProfileResult{}, errors.New("organization_not_authorized")
	}
	ids, err := newIDs(s.ids, 4)
	if err != nil {
		return ProfileResult{}, err
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ProfileResult{}, errors.New("organization_time_unavailable")
	}
	request, _ := json.Marshal(struct {
		Expected int64
		Fields   CompanyProfileFields
	}{in.ExpectedVersion, in.Fields})
	result, err := s.store.SaveProfile(ctx, SaveProfileCommit{CommandID: ids[0], ProfileID: ids[1], AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, Actor: a, ExpectedVersion: in.ExpectedVersion, Fields: in.Fields, KeyHash: sha256.Sum256([]byte("organization-profile-save:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("organization-profile-save:v1:"), request...)), At: now})
	if err != nil {
		return ProfileResult{}, mapStoreError(err)
	}
	return result, nil
}
func (s *Service) CreateLetterhead(ctx context.Context, in CreateLetterheadInput) (LetterheadResult, error) {
	if ctx == nil || !validSubject(in.Subject) || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || in.ExpectedConfigurationVersion < 1 || !validLetterhead(in.Fields) {
		return LetterheadResult{}, errors.New("letterhead_invalid")
	}
	a, err := s.auth.Authorize(ctx, in.Subject, "tenderpreneur.organization.letterhead.manage")
	if err != nil {
		return LetterheadResult{}, errors.New("organization_not_authorized")
	}
	canonical, _ := json.Marshal(in.Fields)
	request, _ := json.Marshal(struct {
		Expected int64
		Fields   LetterheadFields
	}{in.ExpectedConfigurationVersion, in.Fields})
	ids, err := newIDs(s.ids, 4)
	if err != nil {
		return LetterheadResult{}, err
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return LetterheadResult{}, errors.New("organization_time_unavailable")
	}
	result, err := s.store.CreateLetterhead(ctx, CreateLetterheadCommit{CommandID: ids[0], VersionID: ids[1], AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, Actor: a, ExpectedConfigurationVersion: in.ExpectedConfigurationVersion, Fields: in.Fields, ContentHash: sha256.Sum256(canonical), KeyHash: sha256.Sum256([]byte("letterhead-create:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("letterhead-create:v1:"), request...)), At: now})
	if err != nil {
		return LetterheadResult{}, mapStoreError(err)
	}
	return result, nil
}
func (s *Service) ActivateLetterhead(ctx context.Context, in ActivateLetterheadInput) (ActivationResult, error) {
	if ctx == nil || !validSubject(in.Subject) || in.VersionID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || in.ExpectedConfigurationVersion < 1 || !bounded(in.Reason, 1, 2000) {
		return ActivationResult{}, errors.New("letterhead_activation_invalid")
	}
	a, err := s.auth.Authorize(ctx, in.Subject, "tenderpreneur.organization.letterhead.manage")
	if err != nil {
		return ActivationResult{}, errors.New("organization_not_authorized")
	}
	ids, err := newIDs(s.ids, 4)
	if err != nil {
		return ActivationResult{}, err
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return ActivationResult{}, errors.New("organization_time_unavailable")
	}
	request, _ := json.Marshal(struct {
		Version  ID
		Expected int64
		Reason   string
	}{in.VersionID, in.ExpectedConfigurationVersion, in.Reason})
	result, err := s.store.ActivateLetterhead(ctx, ActivateLetterheadCommit{CommandID: ids[0], ActivationID: ids[1], AuditID: ids[2], OutboxID: ids[3], VersionID: in.VersionID, CorrelationID: in.CorrelationID, Actor: a, ExpectedConfigurationVersion: in.ExpectedConfigurationVersion, Reason: in.Reason, KeyHash: sha256.Sum256([]byte("letterhead-activate:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("letterhead-activate:v1:"), request...)), At: now})
	if err != nil {
		return ActivationResult{}, mapStoreError(err)
	}
	return result, nil
}
func (s *Service) GetProfile(ctx context.Context, subject Subject) (CompanyProfile, error) {
	a, err := s.authorizeRead(ctx, subject)
	if err != nil {
		return CompanyProfile{}, err
	}
	v, err := s.store.GetProfile(ctx, a)
	return v, mapReadError(err)
}
func (s *Service) GetLetterhead(ctx context.Context, subject Subject, id ID) (LetterheadVersion, error) {
	if id.IsZero() {
		return LetterheadVersion{}, errors.New("letterhead_invalid")
	}
	a, err := s.authorize(ctx, subject, "tenderpreneur.organization.letterhead.read")
	if err != nil {
		return LetterheadVersion{}, err
	}
	v, err := s.store.GetLetterhead(ctx, a, id)
	return v, mapReadError(err)
}
func (s *Service) GetActiveLetterhead(ctx context.Context, subject Subject) (LetterheadVersion, LetterheadConfiguration, error) {
	a, err := s.authorize(ctx, subject, "tenderpreneur.organization.letterhead.read")
	if err != nil {
		return LetterheadVersion{}, LetterheadConfiguration{}, err
	}
	v, c, err := s.store.GetActiveLetterhead(ctx, a)
	if err != nil {
		return v, c, mapReadError(err)
	}
	return v, c, nil
}
func (s *Service) authorizeRead(ctx context.Context, subject Subject) (Authority, error) {
	return s.authorize(ctx, subject, "tenderpreneur.organization.profile.read")
}
func (s *Service) authorize(ctx context.Context, subject Subject, capability string) (Authority, error) {
	if ctx == nil || !validSubject(subject) || capability == "" {
		return Authority{}, errors.New("organization_read_invalid")
	}
	a, err := s.auth.Authorize(ctx, subject, capability)
	if err != nil {
		return Authority{}, errors.New("organization_not_authorized")
	}
	return a, nil
}
func validSubject(v Subject) bool {
	return !v.IdentityID.IsZero() && !v.TenantID.IsZero() && v.ContextGeneration > 0
}
func bounded(v string, min, max int) bool {
	return len(v) >= min && len(v) <= max && strings.TrimSpace(v) == v && strings.TrimSpace(v) != ""
}
func optional(v *string, max int) bool { return v == nil || bounded(*v, 1, max) }

var color = regexp.MustCompile(`^#[0-9A-F]{6}$`)

func validProfile(v CompanyProfileFields) bool {
	return bounded(v.RegisteredName, 1, 300) && optional(v.TradingName, 300) && bounded(v.BusinessType, 1, 100) && bounded(v.PrimaryIndustry, 1, 200) && bounded(v.Description, 1, 4000) && bounded(v.ServicesCapabilities, 1, 10000) && bounded(v.WorkEmail, 3, 254) && !strings.ContainsAny(v.WorkEmail, " \t\r\n") && bounded(v.WorkPhone, 3, 40) && bounded(v.PreferredLanguage, 2, 35) && bounded(v.Locale, 2, 35) && optional(v.Website, 2048) && optional(v.PostalAddress, 1000) && optional(v.PhysicalAddress, 1000) && optional(v.RegistrationIdentifier, 200) && optional(v.TaxIdentifier, 200)
}
func validLetterhead(v LetterheadFields) bool {
	return bounded(v.RegisteredName, 1, 300) && optional(v.TradingName, 300) && optional(v.RegistrationIdentifier, 200) && optional(v.TaxIdentifier, 200) && optional(v.PostalAddress, 1000) && optional(v.PhysicalAddress, 1000) && bounded(v.Phone, 3, 40) && bounded(v.Email, 3, 254) && !strings.ContainsAny(v.Email, " \t\r\n") && optional(v.Website, 2048) && optional(v.Slogan, 300) && (v.HeaderLayout == "COMPACT" || v.HeaderLayout == "STANDARD" || v.HeaderLayout == "FORMAL") && (v.FooterLayout == "MINIMAL" || v.FooterLayout == "CONTACT" || v.FooterLayout == "LEGAL") && (v.PageNumberStyle == "NONE" || v.PageNumberStyle == "PAGE_X" || v.PageNumberStyle == "PAGE_X_OF_Y") && color.MatchString(v.PrimaryColor) && color.MatchString(v.SecondaryColor) && optional(v.Disclaimer, 2000) && v.TemplateKey == "STANDARD_PROPOSAL"
}
func newIDs(g IDGenerator, n int) ([]ID, error) {
	v := make([]ID, n)
	for i := range v {
		id, err := g.NewID()
		if err != nil || id.IsZero() {
			return nil, errors.New("organization_id_unavailable")
		}
		v[i] = id
	}
	return v, nil
}
func mapStoreError(err error) error {
	if errors.Is(err, ErrConflict) {
		return ErrConflict
	}
	return err
}
func mapReadError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return err
}
