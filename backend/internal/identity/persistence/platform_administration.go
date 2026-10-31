package persistence

import (
	"context"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PlatformAdministrationIdentityTx is the Identity-owned transaction-scoped
// adapter used by the Platform Administration control-plane unit of work.
type PlatformAdministrationIdentityTx struct{ queries *identitysql.Queries }

func NewPlatformAdministrationIdentityTx(db postgres.DBTX) (*PlatformAdministrationIdentityTx, error) {
	if db == nil {
		return nil, errors.New("platform_identity_tx_invalid")
	}
	return &PlatformAdministrationIdentityTx{queries: identitysql.New(db)}, nil
}

func (tx *PlatformAdministrationIdentityTx) LockBootstrapCandidate(ctx context.Context, userID identityapp.ID) (identityapp.PlatformIdentityCandidate, error) {
	row, err := tx.queries.LockPlatformBootstrapIdentityCandidate(ctx, platformIdentityID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return identityapp.PlatformIdentityCandidate{}, identityapp.ErrPlatformIdentityIneligible
	}
	if err != nil {
		return identityapp.PlatformIdentityCandidate{}, err
	}
	return identityapp.PlatformIdentityCandidate{UserID: platformIdentityAppID(row.ID), AuthEpoch: row.AuthEpoch, AffiliationKey: row.EmailLookup}, nil
}

func (tx *PlatformAdministrationIdentityTx) LockInvitee(ctx context.Context, userID, sessionID identityapp.ID, emailLookup string, observedAt time.Time) (identityapp.PlatformIdentityCandidate, error) {
	row, err := tx.queries.LockPlatformInviteeIdentity(ctx, identitysql.LockPlatformInviteeIdentityParams{
		UserID: platformIdentityID(userID), EmailLookup: emailLookup,
		SessionID: platformIdentityID(sessionID), ObservedAt: platformTimestamp(observedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return identityapp.PlatformIdentityCandidate{}, identityapp.ErrPlatformIdentityIneligible
	}
	if err != nil {
		return identityapp.PlatformIdentityCandidate{}, err
	}
	return identityapp.PlatformIdentityCandidate{UserID: platformIdentityAppID(row.ID), AuthEpoch: row.AuthEpoch, AffiliationKey: row.EmailLookup}, nil
}

func (tx *PlatformAdministrationIdentityTx) CountEffective(ctx context.Context, userIDs []identityapp.ID) (int64, error) {
	values := make([]pgtype.UUID, len(userIDs))
	for index, id := range userIDs {
		if id.IsZero() {
			return 0, errors.New("platform_owner_identity_invalid")
		}
		values[index] = platformIdentityID(id)
	}
	return tx.queries.CountEffectivePlatformIdentities(ctx, values)
}

// FindAdministratorByEmailLookup resolves only a verified Platform identity.
// Absence is not distinguished from an identity of a different account type.
func (tx *PlatformAdministrationIdentityTx) FindAdministratorByEmailLookup(ctx context.Context, lookup string) (identityapp.ID, bool, error) {
	id, err := tx.queries.FindPlatformAdministratorByEmailLookup(ctx, lookup)
	if errors.Is(err, pgx.ErrNoRows) {
		return identityapp.ID{}, false, nil
	}
	if err != nil {
		return identityapp.ID{}, false, err
	}
	if !id.Valid {
		return identityapp.ID{}, false, errors.New("platform_identity_search_invalid")
	}
	return identityapp.ID(id.Bytes), true, nil
}

func (tx *PlatformAdministrationIdentityTx) ListAdministratorSummaries(ctx context.Context, userIDs []identityapp.ID) ([]identityapp.PlatformAdministratorIdentitySummary, error) {
	ids := make([]pgtype.UUID, len(userIDs))
	for i, id := range userIDs {
		ids[i] = platformIdentityID(id)
	}
	rows, err := tx.queries.ListPlatformAdministratorIdentitySummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]identityapp.PlatformAdministratorIdentitySummary, len(rows))
	for i, row := range rows {
		if !row.ID.Valid {
			return nil, errors.New("platform_identity_summary_invalid")
		}
		result[i] = identityapp.PlatformAdministratorIdentitySummary{UserID: identityapp.ID(row.ID.Bytes), EmailDisplay: row.Email, Disabled: row.DisabledAt.Valid, TOTPEnrolled: row.TotpEnrolled}
	}
	return result, nil
}

func (tx *PlatformAdministrationIdentityTx) LockAdministrator(ctx context.Context, userID identityapp.ID) (identityapp.PlatformAdministratorIdentity, error) {
	row, err := tx.queries.LockPlatformAdministratorIdentity(ctx, platformIdentityID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return identityapp.PlatformAdministratorIdentity{}, identityapp.ErrPlatformIdentityNotFound
	}
	if err != nil {
		return identityapp.PlatformAdministratorIdentity{}, err
	}
	if !row.ID.Valid || row.Email == "" || row.EmailLookup == "" || row.AuthEpoch < 1 {
		return identityapp.PlatformAdministratorIdentity{}, errors.New("platform_administrator_identity_result_invalid")
	}
	return identityapp.PlatformAdministratorIdentity{UserID: platformIdentityAppID(row.ID), EmailDisplay: row.Email, AffiliationKey: row.EmailLookup, AuthEpoch: row.AuthEpoch, Disabled: row.DisabledAt.Valid}, nil
}

func (tx *PlatformAdministrationIdentityTx) SetDisabled(ctx context.Context, userID identityapp.ID, expectedAuthEpoch int64, at time.Time) (int64, error) {
	changed, err := tx.queries.SetPlatformIdentityDisabled(ctx, identitysql.SetPlatformIdentityDisabledParams{At: platformTimestamp(at), UserID: platformIdentityID(userID), ExpectedAuthEpoch: expectedAuthEpoch})
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, identityapp.ErrPlatformIdentityConflict
	}
	return expectedAuthEpoch + 1, nil
}
func (tx *PlatformAdministrationIdentityTx) SetActive(ctx context.Context, userID identityapp.ID, expectedAuthEpoch int64, at time.Time) (int64, error) {
	changed, err := tx.queries.SetPlatformIdentityActive(ctx, identitysql.SetPlatformIdentityActiveParams{At: platformTimestamp(at), UserID: platformIdentityID(userID), ExpectedAuthEpoch: expectedAuthEpoch})
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, identityapp.ErrPlatformIdentityConflict
	}
	return expectedAuthEpoch + 1, nil
}

func (tx *PlatformAdministrationIdentityTx) AdvanceEpoch(ctx context.Context, userID identityapp.ID, expectedAuthEpoch int64, at time.Time) (int64, error) {
	changed, err := tx.queries.AdvancePlatformIdentityEpoch(ctx, identitysql.AdvancePlatformIdentityEpochParams{At: platformTimestamp(at), UserID: platformIdentityID(userID), ExpectedAuthEpoch: expectedAuthEpoch})
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, identityapp.ErrPlatformIdentityConflict
	}
	return expectedAuthEpoch + 1, nil
}

func (tx *PlatformAdministrationIdentityTx) Promote(ctx context.Context, userID identityapp.ID, expectedAuthEpoch int64, at time.Time) (int64, error) {
	changed, err := tx.queries.PromotePlatformIdentity(ctx, identitysql.PromotePlatformIdentityParams{At: platformTimestamp(at), UserID: platformIdentityID(userID), ExpectedAuthEpoch: expectedAuthEpoch})
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, identityapp.ErrPlatformIdentityConflict
	}
	return expectedAuthEpoch + 1, nil
}

func (tx *PlatformAdministrationIdentityTx) RevokeSessions(ctx context.Context, userID identityapp.ID, at time.Time) (int64, error) {
	return tx.queries.RevokePlatformIdentitySessions(ctx, identitysql.RevokePlatformIdentitySessionsParams{At: platformTimestamp(at), UserID: platformIdentityID(userID)})
}

func (tx *PlatformAdministrationIdentityTx) GetAuthority(ctx context.Context, query identityapp.PlatformAuthorityQuery) (identityapp.PlatformIdentityAuthority, error) {
	row, err := tx.queries.GetPlatformIdentitySessionFacts(ctx, identitysql.GetPlatformIdentitySessionFactsParams{UserID: platformIdentityID(query.UserID), SessionID: platformIdentityID(query.SessionID), ObservedAt: platformTimestamp(query.ObservedAt)})
	if errors.Is(err, pgx.ErrNoRows) {
		return identityapp.PlatformIdentityAuthority{}, identityapp.ErrPlatformIdentityNotFound
	}
	if err != nil {
		return identityapp.PlatformIdentityAuthority{}, err
	}
	if !row.UserID.Valid || row.AuthEpoch < 1 || row.EmailLookup == "" || row.AssuranceLevel == "" || row.AssuranceMethod == "" || !row.AssuredAt.Valid || !row.IdleExpiresAt.Valid || !row.AbsoluteExpiresAt.Valid {
		return identityapp.PlatformIdentityAuthority{}, identityapp.ErrPlatformIdentityNotFound
	}
	return identityapp.PlatformIdentityAuthority{
		UserID: platformIdentityAppID(row.UserID), AuthEpoch: row.AuthEpoch, AffiliationKey: row.EmailLookup,
		AssuranceLevel: row.AssuranceLevel, AssuranceMethod: row.AssuranceMethod,
		AssuredAt: row.AssuredAt.Time.UTC(), IdleExpiresAt: row.IdleExpiresAt.Time.UTC(), AbsoluteExpiresAt: row.AbsoluteExpiresAt.Time.UTC(),
	}, nil
}

func platformIdentityID(id identityapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: !id.IsZero()}
}
func platformIdentityAppID(id pgtype.UUID) identityapp.ID { return identityapp.ID(id.Bytes) }
func platformTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
