// Package accounttype owns the identity account-type value and portal-routing
// invariants. Account type selects a portal; it never grants authorization.
package accounttype

import "errors"

type Type string

const (
	Unassigned    Type = "UNASSIGNED"
	PlatformAdmin Type = "PLATFORM_ADMIN"
	Procurement   Type = "PROCUREMENT"
	Tenderpreneur Type = "TENDERPRENEUR"
)

func ParsePersisted(value *string) (Type, error) {
	if value == nil {
		return Unassigned, nil
	}
	parsed := Type(*value)
	switch parsed {
	case PlatformAdmin, Procurement, Tenderpreneur:
		return parsed, nil
	default:
		return "", errors.New("account_type_invalid")
	}
}

func ParseSelfSelection(value string) (Type, error) {
	parsed := Type(value)
	if parsed != Procurement && parsed != Tenderpreneur {
		return "", errors.New("account_type_selection_invalid")
	}
	return parsed, nil
}

func (t Type) Portal() (key, path string, err error) {
	switch t {
	case Procurement:
		return "PROCUREMENT", "/procurement", nil
	case Tenderpreneur:
		return "TENDERPRENEUR", "/tenderpreneur", nil
	case PlatformAdmin:
		return "", "", errors.New("platform_access_forbidden")
	case Unassigned:
		return "", "", nil
	default:
		return "", "", errors.New("portal_resolution_unavailable")
	}
}
