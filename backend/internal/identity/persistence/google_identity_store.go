package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// GooglePolicyGuard is supplied by composition from the configuration owner.
// It runs inside the identity commit transaction and returns signup permission.
type GooglePolicyGuard func(context.Context, postgres.DBTX, string) (bool, error)
type GoogleIdentityResolver struct {
	runner writeRunner
	policy GooglePolicyGuard
}

func (r *GoogleIdentityResolver) EnablePolicy(guard GooglePolicyGuard) error {
	if r == nil || guard == nil {
		return errors.New("google_policy_invalid")
	}
	r.policy = guard
	return nil
}

func NewGoogleIdentityResolver(runner writeRunner) (*GoogleIdentityResolver, error) {
	if runner == nil {
		return nil, errors.New("google_identity_resolver_invalid")
	}
	return &GoogleIdentityResolver{runner: runner}, nil
}
func (r *GoogleIdentityResolver) ResolveGoogleIdentity(ctx context.Context, commit identityapp.GoogleIdentityCommit) (identityapp.GoogleIdentityResult, error) {
	if ctx == nil || commit.UserID.IsZero() || commit.ExternalIdentityID.IsZero() || commit.AuditID.IsZero() || commit.CorrelationID.IsZero() || commit.Identity.Subject == "" || commit.Identity.Email.IsZero() || commit.AuthenticatedAt.IsZero() {
		return identityapp.GoogleIdentityResult{}, errors.New("google_identity_commit_invalid")
	}
	if r == nil || r.runner == nil {
		return identityapp.GoogleIdentityResult{}, errors.New("google_identity_resolver_invalid")
	}
	var result identityapp.GoogleIdentityResult
	err := r.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("google_identity_unavailable")
		}
		allowSignup := true
		if r.policy != nil {
			var e error
			allowSignup, e = r.policy(txCtx, db, commit.Identity.Email.Lookup())
			if e != nil {
				return e
			}
		}
		queries := identitysql.New(db)
		existing, err := queries.GetGoogleIdentityForUpdate(txCtx, commit.Identity.Subject)
		if err == nil {
			if !existing.UserID.Valid || existing.DisabledAt.Valid || existing.AuthEpoch < 1 {
				return identityapp.ErrGoogleAuthenticationDenied
			}
			affected, err := queries.TouchGoogleIdentity(txCtx, identitysql.TouchGoogleIdentityParams{AuthenticatedAt: identityTime(commit.AuthenticatedAt), ProviderSubject: commit.Identity.Subject, UserID: existing.UserID})
			if err != nil {
				return err
			}
			if affected != 1 {
				return errors.New("google_identity_touch_conflict")
			}
			result = identityapp.GoogleIdentityResult{UserID: identityapp.ID(existing.UserID.Bytes), AuthEpoch: existing.AuthEpoch}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !allowSignup {
			return identityapp.ErrGoogleAuthenticationDenied
		}
		_, err = queries.GetUserByEmailLookupForGoogleLink(txCtx, commit.Identity.Email.Lookup())
		if err == nil {
			return identityapp.ErrGoogleLinkRequired
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = queries.InsertGoogleUser(txCtx, identitysql.InsertGoogleUserParams{ID: identityID(commit.UserID), Email: commit.Identity.Email.Display(), EmailLookup: commit.Identity.Email.Lookup(), CreatedAt: identityTime(commit.AuthenticatedAt)}); err != nil {
			var databaseError *pgconn.PgError
			if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_email_lookup_key" {
				return identityapp.ErrGoogleLinkRequired
			}
			return err
		}
		if err = queries.InsertGoogleIdentity(txCtx, identitysql.InsertGoogleIdentityParams{ID: identityID(commit.ExternalIdentityID), UserID: identityID(commit.UserID), ProviderSubject: commit.Identity.Subject, EmailAtLink: commit.Identity.Email.Display(), CreatedAt: identityTime(commit.AuthenticatedAt)}); err != nil {
			return err
		}
		if err = queries.InsertGoogleUserRegisteredAudit(txCtx, identitysql.InsertGoogleUserRegisteredAuditParams{ID: identityID(commit.AuditID), TargetID: identityID(commit.UserID), OccurredAt: identityTime(commit.AuthenticatedAt), CorrelationID: identityID(commit.CorrelationID)}); err != nil {
			return err
		}
		result = identityapp.GoogleIdentityResult{UserID: commit.UserID, AuthEpoch: 1, Created: true}
		return nil
	})
	if err != nil {
		return identityapp.GoogleIdentityResult{}, err
	}
	if result.UserID.IsZero() {
		return identityapp.GoogleIdentityResult{}, errors.New("google_identity_result_invalid")
	}
	return result, nil
}
