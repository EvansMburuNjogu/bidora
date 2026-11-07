package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type GoogleLinkCommitter struct {
	runner writeRunner
	policy GooglePolicyGuard
}

func (c *GoogleLinkCommitter) EnablePolicy(guard GooglePolicyGuard) error {
	if c == nil || guard == nil {
		return errors.New("google_policy_invalid")
	}
	c.policy = guard
	return nil
}

func NewGoogleLinkCommitter(runner writeRunner) (*GoogleLinkCommitter, error) {
	if runner == nil {
		return nil, errors.New("google_link_committer_invalid")
	}
	return &GoogleLinkCommitter{runner: runner}, nil
}
func (c *GoogleLinkCommitter) CommitGoogleLink(ctx context.Context, commit identityapp.GoogleLinkCommit) error {
	if ctx == nil || commit.UserID.IsZero() || commit.CurrentSessionID.IsZero() || commit.ExternalIdentityID.IsZero() || commit.LinkAuditID.IsZero() || commit.ReplacementSessionID.IsZero() || commit.LoginAuditID.IsZero() || commit.CorrelationID.IsZero() || commit.ExpectedAuthEpoch < 1 || commit.ExpectedAuthEpoch == math.MaxInt64 || commit.NewAuthEpoch < 2 || commit.ExpectedCredentialVersion < 1 || commit.NewAuthEpoch != commit.ExpectedAuthEpoch+1 || commit.Identity.Subject == "" || commit.Identity.Email.IsZero() || commit.SessionHash == [32]byte{} || commit.CSRFHash == [32]byte{} || commit.LinkedAt.IsZero() {
		return errors.New("google_link_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("google_link_committer_invalid")
	}
	idle, absolute, err := session.Expiries(commit.LinkedAt)
	if err != nil || !idle.Equal(commit.IdleExpiresAt) || !absolute.Equal(commit.AbsoluteExpiresAt) {
		return errors.New("google_link_commit_invalid")
	}
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("google_link_unavailable")
		}
		if c.policy != nil {
			if _, e := c.policy(txCtx, db, commit.Identity.Email.Lookup()); e != nil {
				return e
			}
		}
		queries := identitysql.New(db)
		principal, err := queries.GetGoogleLinkPrincipalForUpdate(txCtx, identitysql.GetGoogleLinkPrincipalForUpdateParams{SessionID: identityID(commit.CurrentSessionID), UserID: identityID(commit.UserID), ExpectedAuthEpoch: commit.ExpectedAuthEpoch, ExpectedCredentialVersion: commit.ExpectedCredentialVersion, ObservedAt: identityTime(commit.LinkedAt)})
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrGoogleLinkConflict
		}
		if err != nil {
			return err
		}
		if principal.AuthEpoch != commit.ExpectedAuthEpoch || principal.CredentialVersion != commit.ExpectedCredentialVersion {
			return identityapp.ErrGoogleLinkConflict
		}
		if _, err = queries.GetGoogleIdentityForUpdate(txCtx, commit.Identity.Subject); err == nil {
			return identityapp.ErrGoogleLinkConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = queries.GetGoogleIdentityForUserForUpdate(txCtx, identityID(commit.UserID)); err == nil {
			return identityapp.ErrGoogleLinkConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = queries.InsertGoogleIdentity(txCtx, identitysql.InsertGoogleIdentityParams{ID: identityID(commit.ExternalIdentityID), UserID: identityID(commit.UserID), ProviderSubject: commit.Identity.Subject, EmailAtLink: commit.Identity.Email.Display(), CreatedAt: identityTime(commit.LinkedAt)}); err != nil {
			return linkDatabaseError(err)
		}
		rows, err := queries.AdvanceGoogleLinkAuthEpoch(txCtx, identitysql.AdvanceGoogleLinkAuthEpochParams{UpdatedAt: identityTime(commit.LinkedAt), UserID: identityID(commit.UserID), ExpectedAuthEpoch: commit.ExpectedAuthEpoch})
		if err != nil {
			return err
		}
		if rows != 1 {
			return identityapp.ErrGoogleLinkConflict
		}
		revoked, err := queries.RevokeAllUserSessions(txCtx, identitysql.RevokeAllUserSessionsParams{RevokedAt: identityTime(commit.LinkedAt), UserID: identityID(commit.UserID)})
		if err != nil {
			return err
		}
		if revoked < 1 {
			return identityapp.ErrGoogleLinkConflict
		}
		if err = queries.InsertSession(txCtx, identitysql.InsertSessionParams{ID: identityID(commit.ReplacementSessionID), UserID: identityID(commit.UserID), TokenHash: commit.SessionHash[:], CsrfHash: commit.CSRFHash[:], AuthEpoch: commit.NewAuthEpoch, RotatedFromSessionID: identityID(commit.CurrentSessionID), CreatedAt: identityTime(commit.LinkedAt), IdleExpiresAt: identityTime(commit.IdleExpiresAt), AbsoluteExpiresAt: identityTime(commit.AbsoluteExpiresAt), AssuranceLevel: "AAL1", IdleTimeoutSeconds: int32(commit.IdleExpiresAt.Sub(commit.LinkedAt).Seconds()), AbsoluteTimeoutSeconds: int32(commit.AbsoluteExpiresAt.Sub(commit.LinkedAt).Seconds()), AssuredAt: identityTime(commit.LinkedAt), AssuranceMethod: "PRIMARY_ONLY"}); err != nil {
			return err
		}
		if err = queries.InsertGoogleIdentityLinkedAudit(txCtx, identitysql.InsertGoogleIdentityLinkedAuditParams{ID: identityID(commit.LinkAuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.ExternalIdentityID), OccurredAt: identityTime(commit.LinkedAt), CorrelationID: identityID(commit.CorrelationID)}); err != nil {
			return err
		}
		summary, _ := json.Marshal(struct {
			AuthEpoch            int64                            `json:"auth_epoch"`
			AuthenticationMethod identityapp.AuthenticationMethod `json:"authentication_method"`
			Reason               string                           `json:"reason"`
		}{commit.NewAuthEpoch, identityapp.AuthenticationMethodGoogle, "GOOGLE_IDENTITY_LINKED"})
		return queries.InsertLoginAudit(txCtx, identitysql.InsertLoginAuditParams{ID: identityID(commit.LoginAuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.ReplacementSessionID), OccurredAt: identityTime(commit.LinkedAt), CorrelationID: identityID(commit.CorrelationID), ChangeSummary: summary})
	})
}
func linkDatabaseError(err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return identityapp.ErrGoogleLinkConflict
	}
	return err
}
