package identity_test

import (
	app "bidora/backend/internal/identity/application"
	"context"
	"testing"
	"time"
)

func TestSessionSignInTimeIsTrustedStoredCreationIndependentOfAssuranceAndExpiry(t *testing.T) {
	at := time.Now().UTC()
	pair := sessionPair(t)
	for _, expiry := range []time.Duration{time.Hour, 7 * 24 * time.Hour} {
		record := validSessionRecord(at)
		record.AuthenticatedAt = at.Add(-time.Hour)
		record.AbsoluteExpiresAt = at.Add(expiry)
		record.AssuredAt = at.Add(-time.Minute)
		record.AssuranceLevel = "AAL2"
		record.AssuranceMethod = "EMAIL_OTP"
		reader := &sessionRepositoryFake{record: record}
		validator, _ := app.NewSessionValidator(reader, sessionClockFake{at})
		principal, err := validator.Validate(context.Background(), pair.Session.RevealForCookie())
		if err != nil || !principal.AuthenticatedAt.Equal(record.AuthenticatedAt) || !principal.AssuredAt.Equal(record.AssuredAt) {
			t.Fatal("stored authentication time replaced with expiry or refreshed assurance", err)
		}
	}
	for _, creation := range []time.Time{{}, at.Add(time.Minute)} {
		reader := &sessionRepositoryFake{record: validSessionRecord(at)}
		reader.record.AuthenticatedAt = creation
		validator, _ := app.NewSessionValidator(reader, sessionClockFake{at})
		if _, err := validator.Validate(context.Background(), pair.Session.RevealForCookie()); err == nil {
			t.Fatal("untrusted/missing authentication timestamp accepted")
		}
	}
}
