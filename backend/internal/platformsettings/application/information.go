package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformsettings/domain"
)

var (
	ErrInformationInvalid     = errors.New("platform_information_invalid")
	ErrInformationForbidden   = errors.New("platform_information_forbidden")
	ErrInformationConflict    = errors.New("platform_information_conflict")
	ErrInformationUnavailable = errors.New("platform_information_unavailable")
)

type Information struct {
	PlatformName, ContactPhone string
	SupportEmail, Website      *string
	Version                    int64
	UpdatedAt                  *time.Time
}
type InformationSave struct {
	PlatformName, ContactPhone *string
	Subject                    Subject
	SupportEmail, Website      string
	ExpectedVersion            int64
	IdempotencyKey             string
	CorrelationID              domain.ID
}
type InformationCommand struct {
	RequestHash   [32]byte
	ResultVersion int64
}
type InformationTx interface {
	AuthorizeInformation(context.Context, Subject, time.Time) (Authority, error)
	ReadInformation(context.Context, bool) (Information, error)
	FindInformationCommand(context.Context, domain.ID, [32]byte) (InformationCommand, bool, error)
	UpdateInformation(context.Context, InformationSave, time.Time) (int64, error)
	AppendInformationEvidence(context.Context, domain.ID, domain.ID, InformationSave, int64, time.Time, [32]byte, [32]byte) error
}
type InformationUnitOfWork interface {
	WithinInformationRead(context.Context, func(context.Context, InformationTx) error) error
	WithinInformationWrite(context.Context, func(context.Context, InformationTx) error) error
}
type InformationService struct {
	uow   InformationUnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewInformationService(uow InformationUnitOfWork, ids IDGenerator, clock Clock) (*InformationService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrInformationInvalid
	}
	return &InformationService{uow: uow, ids: ids, clock: clock}, nil
}
func (s *InformationService) Read(ctx context.Context, subject Subject) (result Information, err error) {
	if !validInformationSubject(ctx, subject) || s == nil || s.uow == nil || s.clock == nil {
		return result, ErrInformationInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrInformationUnavailable
	}
	err = s.uow.WithinInformationRead(ctx, func(c context.Context, tx InformationTx) error {
		auth, e := tx.AuthorizeInformation(c, subject, at)
		if e != nil {
			return e
		}
		if !canReadInformation(auth, at) {
			return ErrInformationForbidden
		}
		result, e = tx.ReadInformation(c, false)
		return e
	})
	if err != nil && !errors.Is(err, ErrInformationForbidden) {
		return Information{}, ErrInformationUnavailable
	}
	return result, err
}

// Shared exposes only the intentionally shared platform identity/contact
// projection. The HTTP boundary requires a current Origin-bound session; no
// tenant selector or administrator authority is used for these non-private facts.
func (s *InformationService) Shared(ctx context.Context) (result Information, err error) {
	if ctx == nil || s == nil || s.uow == nil {
		return result, ErrInformationInvalid
	}
	err = s.uow.WithinInformationRead(ctx, func(c context.Context, tx InformationTx) error {
		var e error
		result, e = tx.ReadInformation(c, false)
		return e
	})
	if err != nil {
		return Information{}, ErrInformationUnavailable
	}
	return result, nil
}

func (s *InformationService) Save(ctx context.Context, in InformationSave) (result Information, err error) {
	if s == nil || s.uow == nil || s.ids == nil || s.clock == nil || !validInformationSubject(ctx, in.Subject) || in.ExpectedVersion < 1 || in.CorrelationID.IsZero() || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 || !validSupportWebsite(in.SupportEmail, in.Website) || in.PlatformName != nil && !validPlatformName(*in.PlatformName) || in.ContactPhone != nil && !validContactPhone(*in.ContactPhone) {
		return result, ErrInformationInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrInformationUnavailable
	}
	audit, e := s.ids.NewID()
	if e != nil || audit.IsZero() {
		return result, ErrInformationUnavailable
	}
	outbox, e := s.ids.NewID()
	if e != nil || outbox.IsZero() || outbox == audit {
		return result, ErrInformationUnavailable
	}
	keyHash := sha256.Sum256([]byte("platform-information-key-v1\x00" + in.IdempotencyKey))
	encoded, e := json.Marshal(struct {
		Email, Website string
		Name, Phone    *string
		Version        int64
	}{in.SupportEmail, in.Website, in.PlatformName, in.ContactPhone, in.ExpectedVersion})
	if e != nil {
		return result, ErrInformationInvalid
	}
	requestHash := sha256.Sum256(append([]byte("platform-information-request-v2\x00"), encoded...))
	if in.PlatformName == nil && in.ContactPhone == nil {
		// Preserve committed v1 receipts for existing two-field clients.
		legacy, err := json.Marshal(struct {
			Email, Website string
			Version        int64
		}{in.SupportEmail, in.Website, in.ExpectedVersion})
		if err != nil {
			return result, ErrInformationInvalid
		}
		requestHash = sha256.Sum256(append([]byte("platform-information-request-v1\x00"), legacy...))
	}
	err = s.uow.WithinInformationWrite(ctx, func(c context.Context, tx InformationTx) error {
		auth, e := tx.AuthorizeInformation(c, in.Subject, at)
		if e != nil {
			return e
		}
		if !canReadInformation(auth, at) || !auth.EffectiveOwner {
			return ErrInformationForbidden
		}
		current, e := tx.ReadInformation(c, true)
		if e != nil {
			return e
		}
		previous, found, e := tx.FindInformationCommand(c, in.Subject.ActorID, keyHash)
		if e != nil {
			return e
		}
		if found {
			if previous.RequestHash != requestHash || previous.ResultVersion != current.Version {
				return ErrInformationConflict
			}
			result = current
			return nil
		}
		if current.Version != in.ExpectedVersion {
			return ErrInformationConflict
		}
		name, phone := current.PlatformName, current.ContactPhone
		if in.PlatformName != nil {
			name = *in.PlatformName
		}
		if in.ContactPhone != nil {
			phone = *in.ContactPhone
		}
		in.PlatformName, in.ContactPhone = &name, &phone
		affected, e := tx.UpdateInformation(c, in, at)
		if e != nil {
			return e
		}
		if affected != 1 {
			return ErrInformationConflict
		}
		if e = tx.AppendInformationEvidence(c, audit, outbox, in, current.Version+1, at, keyHash, requestHash); e != nil {
			return e
		}
		result = Information{PlatformName: *in.PlatformName, ContactPhone: *in.ContactPhone, SupportEmail: &in.SupportEmail, Website: &in.Website, Version: current.Version + 1, UpdatedAt: &at}
		return nil
	})
	if err != nil && !errors.Is(err, ErrInformationForbidden) && !errors.Is(err, ErrInformationConflict) {
		return Information{}, ErrInformationUnavailable
	}
	return result, err
}
func validPlatformName(v string) bool {
	return utf8.ValidString(v) && utf8.RuneCountInString(v) >= 1 && utf8.RuneCountInString(v) <= 120 && strings.TrimSpace(v) == v && strings.IndexFunc(v, unicode.IsControl) < 0
}
func validContactPhone(v string) bool {
	if v == "" {
		return true
	}
	if len(v) > 32 || strings.TrimSpace(v) != v {
		return false
	}
	digits := 0
	for i, c := range v {
		if c >= '0' && c <= '9' {
			digits++
			continue
		}
		if c == '+' && i == 0 || c == ' ' || c == '(' || c == ')' || c == '-' {
			continue
		}
		return false
	}
	return digits >= 7
}

func validInformationSubject(ctx context.Context, s Subject) bool {
	return ctx != nil && !s.ActorID.IsZero() && !s.SessionID.IsZero() && s.ContextGeneration > 0
}
func canReadInformation(a Authority, at time.Time) bool {
	return a.Capabilities["platform.admin.read"] && a.AssuranceLevel == "AAL2" && a.AssuranceMethod == "TOTP" && !a.AssuredAt.IsZero() && !a.AssuredAt.After(at) && !a.AssuredAt.Before(at.Add(-15*time.Minute))
}
func validSupportWebsite(mail, site string) bool {
	if _, err := email.Parse(mail); err != nil {
		return false
	}
	if len(site) < 9 || len(site) > 512 || strings.TrimSpace(site) != site || strings.ContainsAny(site, "?#") || !strings.HasPrefix(site, "https://") {
		return false
	}
	for i := 0; i < len(site); i++ {
		if site[i] < 0x21 || site[i] > 0x7e {
			return false
		}
	}
	u, err := url.Parse(site)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.Port() != "" || u.String() != site {
		return false
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if ch != '-' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
				return false
			}
		}
	}
	return true
}
