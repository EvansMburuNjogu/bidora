package pilot

import (
	"errors"

	notificationapp "bidora/backend/internal/notification/application"
	notificationdelivery "bidora/backend/internal/notification/delivery"
	notificationpersistence "bidora/backend/internal/notification/persistence"
	"bidora/backend/internal/notification/platformledger"
	"bidora/backend/internal/notification/tenantemail"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpdelivery"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
)

// BuildInvitationNotificationDelivery composes, but does not activate, tenant
// invitation and Platform invitation/security delivery evidence.
func BuildInvitationNotificationDelivery(control *postgres.Pool, resolver smtpsecret.Resolver, fingerprintKey []byte, fingerprintKeyVersion int32) (*tenantemail.Delivery, *platformledger.Delivery, error) {
	if control == nil || resolver == nil || len(fingerprintKey) < 32 || fingerprintKeyVersion < 1 {
		return nil, nil, errors.New("invitation_notification_composition_invalid")
	}
	store, err := notificationpersistence.NewStore(control)
	if err != nil {
		return nil, nil, err
	}
	evidence, err := notificationapp.NewService(store, notificationIDs{}, systemClock{})
	if err != nil {
		return nil, nil, err
	}
	recorder, err := notificationdelivery.NewRecorder(evidence, notificationdelivery.Config{FingerprintKey: fingerprintKey, FingerprintKeyVersion: fingerprintKeyVersion})
	if err != nil {
		return nil, nil, err
	}
	activeStore, err := settingspersistence.NewActiveSMTPStore(control)
	if err != nil {
		return nil, nil, err
	}
	active, err := smtpdelivery.New(activeStore, resolver)
	if err != nil {
		return nil, nil, err
	}
	tenantDelivery, err := tenantemail.New(active, recorder)
	if err != nil {
		return nil, nil, err
	}
	platformDelivery, err := platformledger.New(active, recorder)
	if err != nil {
		return nil, nil, err
	}
	return tenantDelivery, platformDelivery, nil
}
