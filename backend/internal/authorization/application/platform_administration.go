package application

import "time"

// PlatformAffiliationQuery is the Authorization-owned eligibility lookup used
// by global Platform Administration. AffiliationKey is an internal normalized
// identity key and must never be logged or returned.
type PlatformAffiliationQuery struct {
	UserID         ID
	AffiliationKey string
	ObservedAt     time.Time
}
