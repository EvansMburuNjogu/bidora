package platformsettings_test

import (
	app "bidora/backend/internal/platformsettings/application"
	"errors"
	"testing"
	"time"
)

func TestOperationalPolicyCannotBeBypassedByRouteCaseOrTrailingSlash(t *testing.T) {
	p := validPolicy()
	p.UploadsEnabled = false
	p.RegistrationEnabled = false
	at := time.Now().UTC()
	for _, path := range []string{"/v1/tenderpreneur/opportunities/id/uploads", "/V1/TENDERPRENEUR/OPPORTUNITIES/id/UPLOADS/", "/v1/tenderpreneur/organizations", "/V1/Tenderpreneur/Organizations/"} {
		if !app.OperationalBusinessPath(path) || !errors.Is(p.EnforceBusinessPath(path, at), app.ErrInformationUnavailable) {
			t.Fatal("disabled business route allowed", path)
		}
	}
	p.UploadsEnabled = true
	p.RegistrationEnabled = true
	p.MaintenanceEnabled = true
	p.MaintenanceMessage = "Scheduled maintenance"
	for _, path := range []string{"/v1/tenderpreneur/opportunities", "/V1/TENDERPRENEUR/OPPORTUNITIES/", "/V1/SHARED/INVITATIONS/abc"} {
		var blocked app.OperationalUnavailable
		if !errors.As(p.EnforceBusinessPath(path, at), &blocked) || blocked.PublicMessage() != p.MaintenanceMessage {
			t.Fatal("maintenance bypass", path)
		}
	}
	for _, path := range []string{"/v1/auth/login", "/v1/platform/information", "/v1/shared/platform/information"} {
		if app.OperationalBusinessPath(path) || p.EnforceBusinessPath(path, at) != nil {
			t.Fatal("maintenance locked administrative recovery", path)
		}
	}
}
