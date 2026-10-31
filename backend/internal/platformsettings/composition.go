// Package platformsettings composes the unmounted Platform SMTP settings write boundary.
package platformsettings

import (
	"errors"

	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/openairuntime"
	"bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"bidora/backend/internal/platformsettings/smtptest"
	"bidora/backend/internal/postgres"
)

// ComposeSMTPWriter selects no secret-manager provider and mounts no route. The
// caller must supply an explicit resolver, ID generator and clock.
func ComposeSMTPWriter(pool *postgres.Pool, resolver smtptest.SecretResolver, ids settingsapp.IDGenerator, clock settingsapp.Clock, options ...pesapalruntime.Options) (*settingsapp.Service, error) {
	if pool == nil || resolver == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_smtp_writer_composition_invalid")
	}
	store, err := persistence.NewStore(pool)
	if err != nil {
		return nil, err
	}
	recipients, err := persistence.NewSMTPTestRecipientDirectory(pool)
	if err != nil {
		return nil, err
	}
	informationStore, err := persistence.NewInformationStore(pool)
	if err != nil {
		return nil, err
	}
	information, err := settingsapp.NewInformationService(informationStore, ids, clock)
	if err != nil {
		return nil, err
	}
	tester, err := smtptest.New(resolver, recipients, information)
	if err != nil {
		return nil, err
	}
	pspOptions := pesapalruntime.Options{}
	if len(options) > 0 {
		pspOptions = options[0]
	}
	versions, err := persistence.NewActivePesaPalStore(pool)
	if err != nil {
		return nil, err
	}
	policies, err := persistence.NewRegionalPolicyStore(pool)
	if err != nil {
		return nil, err
	}
	psp, err := pesapalruntime.New(versions, resolver, policies, pspOptions, nil)
	if err != nil {
		return nil, err
	}
	return settingsapp.NewService(store, &openairuntime.Tester{Secrets: resolver, SMTP: tester, PesaPal: psp}, ids, clock)
}
