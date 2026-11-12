package application

import (
	"context"
	"time"
)

// OrganizationCreationStatus is a creator-scoped operational projection; it
// conveys neither active tenant access nor payment or provisioning success.
type OrganizationCreationStatus struct {
	OperationID    ID
	TenantID       ID
	Name           string
	State          string
	JobState       string
	CommercialPath OrganizationCommercialPath
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type organizationCreationStatusReader interface {
	LatestOrganizationCreationStatus(context.Context, ID) (OrganizationCreationStatus, bool, error)
}

func (s *OrganizationCreationService) Status(ctx context.Context, user ID) (result OrganizationCreationStatus, found bool, err error) {
	if s == nil || s.uow == nil || ctx == nil || user.IsZero() {
		return result, false, ErrOrganizationCreationInvalid
	}
	err = s.uow.WithinOrganizationCreation(ctx, func(c context.Context, tx OrganizationCreationTx) error {
		facts, e := tx.LockCreator(c, user)
		if e != nil {
			return e
		}
		if !facts.Verified || facts.Disabled || !facts.Tenderpreneur {
			return ErrOrganizationCreationIneligible
		}
		reader, ok := tx.(organizationCreationStatusReader)
		if !ok {
			return ErrOrganizationCreationUnavailable
		}
		result, found, e = reader.LatestOrganizationCreationStatus(c, user)
		return e
	})
	if err != nil && err != ErrOrganizationCreationIneligible {
		return OrganizationCreationStatus{}, false, ErrOrganizationCreationUnavailable
	}
	return result, found, err
}
