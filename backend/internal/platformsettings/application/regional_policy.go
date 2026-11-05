package application

import (
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

type RequestMetadata struct {
	IP         string    `json:"ip_address"`
	Browser    string    `json:"browser"`
	ObservedAt time.Time `json:"observed_at"`
}
type RestrictedMetadataReader interface {
	RequestMetadata(context.Context, RegionalSubject, domain.ID, time.Time) (RequestMetadata, error)
}

func (s *RegionalPolicyService) RequestMetadata(ctx context.Context, subject RegionalSubject, audit domain.ID) (RequestMetadata, error) {
	if !validInformationSubject(ctx, subject.Subject) || audit.IsZero() {
		return RequestMetadata{}, ErrInformationInvalid
	}
	reader, ok := s.uow.(RestrictedMetadataReader)
	if !ok {
		return RequestMetadata{}, ErrInformationUnavailable
	}
	return reader.RequestMetadata(ctx, subject, audit, s.clock.Now().UTC())
}

type RegionalSubject struct {
	Changes []FieldChange
	Subject
	CorrelationID domain.ID
}

type OperationalUnavailable struct{ Message string }

func (OperationalUnavailable) Error() string           { return "platform_operation_unavailable" }
func (e OperationalUnavailable) PublicMessage() string { return e.Message }

type OperationalAvailability struct {
	MaintenanceActive bool   `json:"maintenance_active"`
	Message           string `json:"maintenance_message"`
}

func (s *RegionalPolicyService) Availability(ctx context.Context) (OperationalAvailability, error) {
	p, e := s.Policy(ctx, RegionalSubject{}, true)
	if e != nil {
		return OperationalAvailability{}, e
	}
	active := p.UnderMaintenance(s.clock.Now().UTC())
	message := ""
	if active {
		message = p.MaintenanceMessage
	}
	return OperationalAvailability{active, message}, nil
}

type OperationalPolicy struct {
	Version               int64      `json:"version"`
	RegistrationEnabled   bool       `json:"organization_registration_enabled"`
	AIEnabled             bool       `json:"ai_features_enabled"`
	UploadsEnabled        bool       `json:"document_uploads_enabled"`
	MaxDocumentBytes      int64      `json:"max_document_bytes"`
	MaintenanceEnabled    bool       `json:"maintenance_enabled"`
	MaintenanceMessage    string     `json:"maintenance_message"`
	MaintenanceStartsAt   *time.Time `json:"maintenance_starts_at"`
	MaintenanceEndsAt     *time.Time `json:"maintenance_ends_at"`
	MetadataRetentionDays int32      `json:"restricted_metadata_retention_days"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (p OperationalPolicy) Valid() bool {
	return p.Version > 0 && p.MaxDocumentBytes >= 1<<20 && p.MaxDocumentBytes <= 25<<20 && p.MetadataRetentionDays >= 7 && p.MetadataRetentionDays <= 90 && utf8.ValidString(p.MaintenanceMessage) && len(p.MaintenanceMessage) <= 240 && !strings.ContainsAny(p.MaintenanceMessage, "\r\n\x00") && (!p.MaintenanceEnabled || strings.TrimSpace(p.MaintenanceMessage) != "") && (p.MaintenanceStartsAt == nil || p.MaintenanceEndsAt == nil || p.MaintenanceEndsAt.After(*p.MaintenanceStartsAt))
}
func (p OperationalPolicy) UnderMaintenance(at time.Time) bool {
	return p.MaintenanceEnabled && (p.MaintenanceStartsAt == nil || !at.Before(*p.MaintenanceStartsAt)) && (p.MaintenanceEndsAt == nil || at.Before(*p.MaintenanceEndsAt))
}

type RegionalReceipt struct {
	Hash    [32]byte
	Version int64
	Target  string
}
type RegionalPolicyTx interface {
	AuthorizeRegional(context.Context, RegionalSubject, string, bool, time.Time) error
	Countries(context.Context) ([]domain.CountryConfiguration, error)
	Country(context.Context, string, bool) (domain.CountryConfiguration, error)
	SaveCountry(context.Context, domain.CountryConfiguration, RegionalSubject, time.Time) error
	CountryHistory(context.Context, string) ([]domain.CountryConfiguration, error)
	Policy(context.Context, bool) (OperationalPolicy, error)
	SavePolicy(context.Context, OperationalPolicy, RegionalSubject, time.Time) error
	PolicyHistory(context.Context) ([]OperationalPolicy, error)
	Command(context.Context, RegionalSubject, string, [32]byte) (RegionalReceipt, bool, error)
	Evidence(context.Context, RegionalSubject, string, string, int64, [32]byte, [32]byte, domain.ID, domain.ID, time.Time) error
}
type RegionalPolicyUnitOfWork interface {
	WithinRegionalPolicy(context.Context, bool, func(context.Context, RegionalPolicyTx) error) error
}
type RegionalPolicyService struct {
	uow   RegionalPolicyUnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewRegionalPolicyService(uow RegionalPolicyUnitOfWork, ids IDGenerator, clock Clock) (*RegionalPolicyService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrInformationInvalid
	}
	return &RegionalPolicyService{uow, ids, clock}, nil
}
func (s *RegionalPolicyService) Countries(ctx context.Context, subject RegionalSubject, shared bool) (out []domain.CountryConfiguration, err error) {
	if s == nil || ctx == nil {
		return nil, ErrInformationInvalid
	}
	err = s.uow.WithinRegionalPolicy(ctx, false, func(c context.Context, tx RegionalPolicyTx) error {
		if !shared {
			if !validInformationSubject(c, subject.Subject) {
				return ErrInformationInvalid
			}
			if e := tx.AuthorizeRegional(c, subject, "COUNTRY", false, s.clock.Now()); e != nil {
				return e
			}
		}
		rows, e := tx.Countries(c)
		for _, row := range rows {
			if !shared || row.Enabled {
				out = append(out, row.WithName())
			}
		}
		return e
	})
	if out == nil {
		out = []domain.CountryConfiguration{}
	}
	return out, googleError(err)
}
func (s *RegionalPolicyService) CountryHistory(ctx context.Context, subject RegionalSubject, code string) (out []domain.CountryConfiguration, err error) {
	if !validInformationSubject(ctx, subject.Subject) {
		return nil, ErrInformationInvalid
	}
	err = s.uow.WithinRegionalPolicy(ctx, false, func(c context.Context, tx RegionalPolicyTx) error {
		if e := tx.AuthorizeRegional(c, subject, "COUNTRY", false, s.clock.Now()); e != nil {
			return e
		}
		var e error
		out, e = tx.CountryHistory(c, code)
		return e
	})
	return out, googleError(err)
}
func (s *RegionalPolicyService) SaveCountry(ctx context.Context, subject RegionalSubject, value domain.CountryConfiguration, key string) (out domain.CountryConfiguration, err error) {
	value.Locale = value.Language + "-" + value.Code
	if !validInformationSubject(ctx, subject.Subject) || !value.Valid() || key == "" || len(key) > 128 {
		return out, ErrInformationInvalid
	}
	value.Name = ""
	value.UpdatedAt = time.Time{}
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	kh := sha256.Sum256([]byte("country-v1:" + key))
	at := s.clock.Now().UTC()
	err = s.uow.WithinRegionalPolicy(ctx, true, func(c context.Context, tx RegionalPolicyTx) error {
		if e := tx.AuthorizeRegional(c, subject, "COUNTRY", true, at); e != nil {
			return e
		}
		old, found, e := tx.Command(c, subject, "COUNTRY", kh)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash || old.Target != value.Code {
				return ErrInformationConflict
			}
			out, e = tx.Country(c, value.Code, false)
			if e == nil && out.Version != old.Version {
				return ErrInformationConflict
			}
			return e
		}
		var previous domain.CountryConfiguration
		if value.Version > 0 {
			previous, e = tx.Country(c, value.Code, true)
			if e != nil {
				return e
			}
		}
		subject.Changes = selectedFieldChanges(previous, value, "country_code", "currency", "language", "timezone", "locale", "enabled", "is_default")
		if e = tx.SaveCountry(c, value, subject, at); e != nil {
			return e
		}
		out, e = tx.Country(c, value.Code, false)
		if e != nil {
			return e
		}
		audit, e := s.ids.NewID()
		if e != nil {
			return e
		}
		event, e := s.ids.NewID()
		if e != nil {
			return e
		}
		return tx.Evidence(c, subject, "COUNTRY", value.Code, out.Version, kh, hash, audit, event, at)
	})
	return out.WithName(), googleError(err)
}
func (s *RegionalPolicyService) Policy(ctx context.Context, subject RegionalSubject, internal bool) (out OperationalPolicy, err error) {
	if s == nil || ctx == nil {
		return out, ErrInformationInvalid
	}
	err = s.uow.WithinRegionalPolicy(ctx, false, func(c context.Context, tx RegionalPolicyTx) error {
		if !internal {
			if !validInformationSubject(c, subject.Subject) {
				return ErrInformationInvalid
			}
			if e := tx.AuthorizeRegional(c, subject, "OPERATIONS", false, s.clock.Now()); e != nil {
				return e
			}
		}
		var e error
		out, e = tx.Policy(c, false)
		if e == nil && !out.Valid() {
			return ErrInformationUnavailable
		}
		return e
	})
	return out, googleError(err)
}
func (s *RegionalPolicyService) PolicyHistory(ctx context.Context, subject RegionalSubject) (out []OperationalPolicy, err error) {
	if !validInformationSubject(ctx, subject.Subject) {
		return nil, ErrInformationInvalid
	}
	err = s.uow.WithinRegionalPolicy(ctx, false, func(c context.Context, tx RegionalPolicyTx) error {
		if e := tx.AuthorizeRegional(c, subject, "OPERATIONS", false, s.clock.Now()); e != nil {
			return e
		}
		var e error
		out, e = tx.PolicyHistory(c)
		return e
	})
	return out, googleError(err)
}
func (s *RegionalPolicyService) SavePolicy(ctx context.Context, subject RegionalSubject, value OperationalPolicy, key string) (out OperationalPolicy, err error) {
	if !validInformationSubject(ctx, subject.Subject) || !value.Valid() || key == "" || len(key) > 128 {
		return out, ErrInformationInvalid
	}
	value.UpdatedAt = time.Time{}
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	kh := sha256.Sum256([]byte("operations-v1:" + key))
	at := s.clock.Now().UTC()
	err = s.uow.WithinRegionalPolicy(ctx, true, func(c context.Context, tx RegionalPolicyTx) error {
		if e := tx.AuthorizeRegional(c, subject, "OPERATIONS", true, at); e != nil {
			return e
		}
		old, found, e := tx.Command(c, subject, "OPERATIONS", kh)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash {
				return ErrInformationConflict
			}
			out, e = tx.Policy(c, false)
			if e == nil && old.Version != out.Version {
				return ErrInformationConflict
			}
			return e
		}
		previous, e := tx.Policy(c, true)
		if e != nil {
			return e
		}
		subject.Changes = selectedFieldChanges(previous, value, "organization_registration_enabled", "ai_features_enabled", "document_uploads_enabled", "max_document_bytes", "maintenance_enabled", "maintenance_message", "maintenance_starts_at", "maintenance_ends_at", "restricted_metadata_retention_days")
		if e = tx.SavePolicy(c, value, subject, at); e != nil {
			return e
		}
		out, e = tx.Policy(c, false)
		if e != nil {
			return e
		}
		audit, e := s.ids.NewID()
		if e != nil {
			return e
		}
		event, e := s.ids.NewID()
		if e != nil {
			return e
		}
		return tx.Evidence(c, subject, "OPERATIONS", "policy", out.Version, kh, hash, audit, event, at)
	})
	return out, googleError(err)
}
