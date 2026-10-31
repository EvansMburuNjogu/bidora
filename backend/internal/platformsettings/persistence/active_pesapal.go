package persistence

import (
	"bidora/backend/internal/platformsettings/domain"
	q "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
)

type ActivePesaPalStore struct{ runner settingsReadRunner }

func NewActivePesaPalStore(r settingsReadRunner) (*ActivePesaPalStore, error) {
	if r == nil {
		return nil, errors.New("pesapal_store_invalid")
	}
	return &ActivePesaPalStore{r}, nil
}
func (s *ActivePesaPalStore) ActivePesaPal(ctx context.Context) (out domain.Version, err error) {
	err = s.runner.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		id, e := q.New(db).GetActivePesaPalVersionID(c)
		if e != nil {
			return e
		}
		out, e = readPesaPal(c, q.New(db), domain.ID(id.Bytes))
		if e != nil {
			return e
		}
		if out.State != domain.Active {
			return errors.New("pesapal_not_active")
		}
		return nil
	})
	return out, err
}
func (s *ActivePesaPalStore) PesaPalVersion(ctx context.Context, id domain.ID) (out domain.Version, err error) {
	err = s.runner.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		var e error
		out, e = readPesaPal(c, q.New(db), id)
		return e
	})
	return out, err
}
func readPesaPal(ctx context.Context, queries *q.Queries, ID domain.ID) (domain.Version, error) {
	v, e := queries.GetPesaPalConfiguration(ctx, uuid(ID))
	if e != nil {
		return domain.Version{}, e
	}
	c, hash, e := domain.NewPesaPalConfiguration(domain.PesaPalConfiguration{Enabled: v.Enabled, Environment: v.Environment, MerchantName: v.MerchantName, CredentialsSecretRef: v.CredentialsSecretRef, NotificationID: id(v.NotificationID)})
	if e != nil || hash != hash32(v.ConfigurationHash) || !v.ID.Valid {
		return domain.Version{}, errors.New("pesapal_configuration_corrupt")
	}
	return domain.Version{ID: id(v.ID), Kind: domain.PesaPal, Version: v.Version, State: domain.VersionState(v.State), Configuration: c, ConfigurationHash: hash}, nil
}
