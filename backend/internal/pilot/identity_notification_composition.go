package pilot

import (
	"errors"

	notificationapp "bidora/backend/internal/notification/application"
	notificationdelivery "bidora/backend/internal/notification/delivery"
	notificationdomain "bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/identityemail"
	notificationpersistence "bidora/backend/internal/notification/persistence"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpdelivery"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

// BuildIdentityNotificationDelivery composes, but does not activate, exact-active
// SMTP delivery evidence for verification and recovery email.
func BuildIdentityNotificationDelivery(control *postgres.Pool, resolver smtpsecret.Resolver, fingerprintKey []byte, fingerprintKeyVersion int32) (*identityemail.Delivery, error) {
	if control == nil || resolver == nil || len(fingerprintKey) < 32 || fingerprintKeyVersion < 1 {
		return nil, errors.New("identity_notification_composition_invalid")
	}
	notificationStore, err := notificationpersistence.NewStore(control)
	if err != nil {
		return nil, err
	}
	evidence, err := notificationapp.NewService(notificationStore, notificationIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	recorder, err := notificationdelivery.NewRecorder(evidence, notificationdelivery.Config{FingerprintKey: fingerprintKey, FingerprintKeyVersion: fingerprintKeyVersion})
	if err != nil {
		return nil, err
	}
	activeStore, err := settingspersistence.NewActiveSMTPStore(control)
	if err != nil {
		return nil, err
	}
	active, err := smtpdelivery.New(activeStore, resolver)
	if err != nil {
		return nil, err
	}
	return identityemail.New(active, recorder)
}

type notificationIDs struct{}

func (notificationIDs) NewID() (notificationdomain.ID, error) {
	value, err := uuid.NewRandom()
	return notificationdomain.ID(value), err
}
