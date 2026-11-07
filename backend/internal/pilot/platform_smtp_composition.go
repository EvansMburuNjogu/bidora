package pilot

import (
	"bidora/backend/internal/httpapi"
	platformsettings "bidora/backend/internal/platformsettings"
	settingsapp "bidora/backend/internal/platformsettings/application"
	settingsdomain "bidora/backend/internal/platformsettings/domain"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"bidora/backend/internal/platformsettings/smtptest"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

// BuildPlatformSMTPWriteHandler composes, but does not register, the governed
// SMTP write/test/activation HTTP boundary. No secret-manager provider is chosen.
func BuildPlatformSMTPWriteHandler(pool *postgres.Pool, resolver smtptest.SecretResolver, options ...pesapalruntime.Options) (*httpapi.PlatformIntegrationSettingsWriteHandler, error) {
	writer, err := platformsettings.ComposeSMTPWriter(pool, resolver, platformSettingIDs{}, systemClock{}, options...)
	if err != nil {
		return nil, err
	}
	readStore, err := settingspersistence.NewReadStore(pool)
	if err != nil {
		return nil, err
	}
	reader, err := settingsapp.NewReadService(readStore, systemClock{})
	if err != nil {
		return nil, err
	}
	if usernames, ok := resolver.(settingsapp.SMTPUsernameReader); ok {
		reader.EnableSMTPUsernames(usernames)
	}
	secretWriter, ok := resolver.(httpapi.PlatformSMTPSecretWriter)
	if !ok {
		return nil, settingsapp.ErrUnavailable
	}
	return httpapi.NewPlatformIntegrationSettingsWriteHandlerWithSecrets(writer, reader, systemClock{}, secretWriter)
}

type platformSettingIDs struct{}

func (platformSettingIDs) NewID() (settingsdomain.ID, error) {
	value, err := uuid.NewRandom()
	return settingsdomain.ID(value), err
}
