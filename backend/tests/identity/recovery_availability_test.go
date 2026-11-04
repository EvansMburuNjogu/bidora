package identity_test

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"context"
	"testing"
)

type recoveryAvailability bool

func (a recoveryAvailability) EmailLoginAvailable(context.Context) bool { return bool(a) }

func TestRecoveryDoesNotAcknowledgeUnavailableSMTPOrExposeAccountEligibility(t *testing.T) {
	for _, address := range []string{"known@example.test", "unknown@example.test"} {
		requester := &recoveryRequesterFake{}
		service, err := app.NewPublicPasswordRecoveryService(requester, &recoveryAttemptsFake{}, recoveryKeyFake{key: attempt.Key{1}}, recoveryAvailability(false))
		if err != nil {
			t.Fatal(err)
		}
		err = service.Request(context.Background(), app.PublicPasswordRecoveryInput{Email: address, CorrelationID: identityID(1), NetworkScope: attempt.Key{2}})
		if err == nil || err.Error() != "password_recovery_request_unavailable" || requester.calls != 0 {
			t.Fatal("unavailable SMTP was acknowledged or account lookup occurred")
		}
	}
	requester := &recoveryRequesterFake{}
	service, _ := app.NewPublicPasswordRecoveryService(requester, &recoveryAttemptsFake{}, recoveryKeyFake{key: attempt.Key{1}}, recoveryAvailability(true))
	if err := service.Request(context.Background(), app.PublicPasswordRecoveryInput{Email: "user@example.test", CorrelationID: identityID(1), NetworkScope: attempt.Key{2}}); err != nil || requester.calls != 1 {
		t.Fatal("available recovery did not proceed")
	}
}
