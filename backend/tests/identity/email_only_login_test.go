package identity_test

import (
	"context"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	store "bidora/backend/internal/identity/persistence"
)

func TestEmailOnlyLoginIgnoresEnrolledAuthenticatorsForEveryPrimaryMethod(t *testing.T) {
	for _, method := range []app.AuthenticationMethod{app.AuthenticationMethodPassword, app.AuthenticationMethodGoogle} {
		t.Run(string(method), func(t *testing.T) {
			base := &mfaStoreFake{active: &app.MFAFactorMaterial{FactorID: identityID(3), UserID: identityID(2)}}
			service, _ := mfaServiceFixture(t, time.Now().UTC(), base, &mfaAttemptsFake{threshold: 5})
			mail := &loginMailFake{}
			_ = service.EnableEmailLogin(mail, []byte(strings.Repeat("k", 32)))
			_ = service.EnablePlatformEmailLogin(platformEmailPolicyFake{required: true}, mail)
			service.RequireEmailLogin()
			gate, err := service.BeginLogin(context.Background(), identityID(2), 1, method)
			if err != nil || !gate.Required || gate.Token == "" || mail.calls != 1 || base.loginChallenge.Purpose != "EMAIL_LOGIN" || base.loginChallenge.PrimaryMethod != string(method) {
				t.Fatal("email-only runtime used an authenticator or skipped verification", err)
			}
		})
	}
}

func TestAdministratorGoogleEmailCodeUsesTrustedServerPolicy(t *testing.T) {
	db := &emailAssuranceDB{admin: true, primary: "GOOGLE"}
	s, err := store.NewMFAStore(db)
	if err != nil {
		t.Fatal(err)
	}
	s.EnablePlatformEmailLogin()
	at := time.Now().UTC()
	err = s.CommitMFALogin(context.Background(), app.MFALoginCommit{UserID: identityID(2), TransactionID: identityID(3), SessionID: identityID(4), AuditID: identityID(5), OutboxID: identityID(6), CorrelationID: identityID(7), AuthEpoch: 1, Method: "EMAIL_OTP", PrimaryMethod: "GOOGLE", At: at, IdleExpiresAt: at.Add(time.Minute), AbsoluteExpiresAt: at.Add(time.Hour)})
	if err != nil || db.sessionLevel != "AAL2" || db.auditLevel != "AAL2" {
		t.Fatal("trusted administrator Google email verification failed", err)
	}
}

func TestEmailOnlyLoginWithoutDeliveryNeverFallsBackToPasswordOnly(t *testing.T) {
	service, _ := mfaServiceFixture(t, time.Now().UTC(), &mfaStoreFake{}, &mfaAttemptsFake{threshold: 5})
	service.RequireEmailLogin()
	gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
	if err == nil || gate.Token != "" {
		t.Fatal("missing SMTP bypassed required verification")
	}
}
