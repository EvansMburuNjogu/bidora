package application

import (
	"strings"
	"time"
)

// OperationalBusinessPath matches Fiber's case-insensitive business routing.
func OperationalBusinessPath(path string) bool {
	path = strings.TrimRight(strings.ToLower(path), "/")
	return strings.HasPrefix(path, "/v1/tenderpreneur/") || strings.HasPrefix(path, "/v1/shared/invitations")
}
func (p OperationalPolicy) EnforceBusinessPath(path string, at time.Time) error {
	path = strings.TrimRight(strings.ToLower(path), "/")
	if !OperationalBusinessPath(path) {
		return nil
	}
	if !p.Valid() {
		return ErrInformationUnavailable
	}
	if p.UnderMaintenance(at) {
		return OperationalUnavailable{Message: p.MaintenanceMessage}
	}
	if strings.Contains(path, "/uploads") && !p.UploadsEnabled || path == "/v1/tenderpreneur/organizations" && !p.RegistrationEnabled {
		return ErrInformationUnavailable
	}
	return nil
}
