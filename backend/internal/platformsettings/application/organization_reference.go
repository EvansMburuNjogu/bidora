package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"bidora/backend/internal/platformsettings/domain"
)

type OrganizationReference struct {
	Kind      string    `json:"kind"`
	Code      string    `json:"code"`
	Label     string    `json:"label"`
	Enabled   bool      `json:"enabled"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationReferenceReceipt struct {
	Hash       [32]byte
	Kind, Code string
	Version    int64
}

type OrganizationReferenceTx interface {
	ListOrganizationReferences(context.Context, string, bool) ([]OrganizationReference, error)
	LockOrganizationReference(context.Context, string, string) (OrganizationReference, error)
	SaveOrganizationReference(context.Context, OrganizationReference, RegionalSubject, time.Time) error
	OrganizationReferenceCommand(context.Context, RegionalSubject, [32]byte) (OrganizationReferenceReceipt, bool, error)
	OrganizationReferenceEvidence(context.Context, RegionalSubject, OrganizationReference, [32]byte, [32]byte, domain.ID, domain.ID, time.Time) error
}

var organizationReferenceCode = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func ValidOrganizationReferenceKind(kind string) bool {
	return kind == "BUSINESS_TYPE" || kind == "INDUSTRY"
}
func validOrganizationReference(v OrganizationReference) bool {
	if !ValidOrganizationReferenceKind(v.Kind) || len(v.Code) < 1 || len(v.Code) > 64 || !organizationReferenceCode.MatchString(v.Code) || !utf8.ValidString(v.Label) || len(v.Label) < 1 || len(v.Label) > 100 || strings.TrimSpace(v.Label) != v.Label || v.Version < 0 {
		return false
	}
	for _, r := range v.Label {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (s *RegionalPolicyService) OrganizationReferences(ctx context.Context, subject RegionalSubject, kind string, shared bool) (out []OrganizationReference, err error) {
	if s == nil || ctx == nil || !ValidOrganizationReferenceKind(kind) {
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
		references, ok := tx.(OrganizationReferenceTx)
		if !ok {
			return ErrInformationUnavailable
		}
		var e error
		out, e = references.ListOrganizationReferences(c, kind, shared)
		return e
	})
	if out == nil {
		out = []OrganizationReference{}
	}
	return out, googleError(err)
}

func (s *RegionalPolicyService) SaveOrganizationReference(ctx context.Context, subject RegionalSubject, value OrganizationReference, key string) (out OrganizationReference, err error) {
	if s == nil || !validInformationSubject(ctx, subject.Subject) || subject.CorrelationID.IsZero() || !validOrganizationReference(value) || key == "" || len(key) > 128 {
		return out, ErrInformationInvalid
	}
	value.UpdatedAt = time.Time{}
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	kh := sha256.Sum256([]byte("organization-reference:v1:" + key))
	at := s.clock.Now().UTC()
	err = s.uow.WithinRegionalPolicy(ctx, true, func(c context.Context, tx RegionalPolicyTx) error {
		if e := tx.AuthorizeRegional(c, subject, "COUNTRY", true, at); e != nil {
			return e
		}
		refs, ok := tx.(OrganizationReferenceTx)
		if !ok {
			return ErrInformationUnavailable
		}
		old, found, e := refs.OrganizationReferenceCommand(c, subject, kh)
		if e != nil {
			return e
		}
		if found {
			if old.Hash != hash || old.Kind != value.Kind || old.Code != value.Code {
				return ErrInformationConflict
			}
			out, e = refs.LockOrganizationReference(c, value.Kind, value.Code)
			if e == nil && out.Version != old.Version {
				return ErrInformationConflict
			}
			return e
		}
		var previous OrganizationReference
		if value.Version > 0 {
			previous, e = refs.LockOrganizationReference(c, value.Kind, value.Code)
			if e != nil {
				return e
			}
		}
		subject.Changes = selectedFieldChanges(previous, value, "label", "enabled")
		if e = refs.SaveOrganizationReference(c, value, subject, at); e != nil {
			return e
		}
		out, e = refs.LockOrganizationReference(c, value.Kind, value.Code)
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
		return refs.OrganizationReferenceEvidence(c, subject, out, kh, hash, audit, event, at)
	})
	return out, googleError(err)
}
