package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type PasswordChangeCommitter struct{ runner writeRunner }

func NewPasswordChangeCommitter(runner writeRunner) (*PasswordChangeCommitter, error) {
	if runner == nil {
		return nil, errors.New("password_change_committer_invalid")
	}
	return &PasswordChangeCommitter{runner}, nil
}
func (c *PasswordChangeCommitter) CommitPasswordChange(ctx context.Context, commit identityapp.PasswordChangeCommit) error {
	if ctx == nil || commit.UserID.IsZero() || commit.CurrentSessionID.IsZero() || commit.ReplacementSessionID.IsZero() || commit.PasswordAuditID.IsZero() || commit.LoginAuditID.IsZero() || commit.CorrelationID.IsZero() || commit.ExpectedAuthEpoch < 1 || commit.ExpectedAuthEpoch == math.MaxInt64 || commit.ExpectedCredentialVersion < 1 || commit.ExpectedCredentialVersion == math.MaxInt64 || commit.NewAuthEpoch != commit.ExpectedAuthEpoch+1 || commit.NewCredentialVersion != commit.ExpectedCredentialVersion+1 || commit.PasswordPHC == "" || commit.SessionHash == [32]byte{} || commit.CSRFHash == [32]byte{} || commit.ChangedAt.IsZero() {
		return errors.New("password_change_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("password_change_committer_invalid")
	}
	idle, absolute, err := securitypolicy.Expiries(ctx, commit.ChangedAt, false)
	if err != nil || !idle.Equal(commit.IdleExpiresAt) || !absolute.Equal(commit.AbsoluteExpiresAt) {
		return errors.New("password_change_commit_invalid")
	}
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("password_change_unavailable")
		}
		queries := identitysql.New(db)
		principal, err := queries.GetPasswordChangePrincipalForUpdate(txCtx, identitysql.GetPasswordChangePrincipalForUpdateParams{SessionID: identityID(commit.CurrentSessionID), UserID: identityID(commit.UserID), ExpectedAuthEpoch: commit.ExpectedAuthEpoch, ExpectedCredentialVersion: commit.ExpectedCredentialVersion, ObservedAt: identityTime(commit.ChangedAt)})
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrPasswordChangeConflict
		}
		if err != nil {
			return err
		}
		if principal.AuthEpoch != commit.ExpectedAuthEpoch || principal.CredentialVersion != commit.ExpectedCredentialVersion {
			return identityapp.ErrPasswordChangeConflict
		}
		affected, err := queries.ChangePasswordCredential(txCtx, identitysql.ChangePasswordCredentialParams{PasswordPhc: commit.PasswordPHC, ChangedAt: identityTime(commit.ChangedAt), UserID: identityID(commit.UserID), ExpectedCredentialVersion: commit.ExpectedCredentialVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return identityapp.ErrPasswordChangeConflict
		}
		affected, err = queries.AdvancePasswordChangeAuthEpoch(txCtx, identitysql.AdvancePasswordChangeAuthEpochParams{ChangedAt: identityTime(commit.ChangedAt), UserID: identityID(commit.UserID), ExpectedAuthEpoch: commit.ExpectedAuthEpoch})
		if err != nil {
			return err
		}
		if affected != 1 {
			return identityapp.ErrPasswordChangeConflict
		}
		revoked, err := queries.RevokeAllUserSessions(txCtx, identitysql.RevokeAllUserSessionsParams{RevokedAt: identityTime(commit.ChangedAt), UserID: identityID(commit.UserID)})
		if err != nil {
			return err
		}
		if revoked < 1 {
			return identityapp.ErrPasswordChangeConflict
		}
		if err = queries.InsertSession(txCtx, identitysql.InsertSessionParams{ID: identityID(commit.ReplacementSessionID), UserID: identityID(commit.UserID), TokenHash: commit.SessionHash[:], CsrfHash: commit.CSRFHash[:], AuthEpoch: commit.NewAuthEpoch, RotatedFromSessionID: identityID(commit.CurrentSessionID), CreatedAt: identityTime(commit.ChangedAt), IdleExpiresAt: identityTime(commit.IdleExpiresAt), AbsoluteExpiresAt: identityTime(commit.AbsoluteExpiresAt), AssuranceLevel: "AAL1", IdleTimeoutSeconds: int32(commit.IdleExpiresAt.Sub(commit.ChangedAt).Seconds()), AbsoluteTimeoutSeconds: int32(commit.AbsoluteExpiresAt.Sub(commit.ChangedAt).Seconds()), AssuredAt: identityTime(commit.ChangedAt), AssuranceMethod: "PRIMARY_ONLY"}); err != nil {
			return err
		}
		if err = queries.InsertPasswordChangedAudit(txCtx, identitysql.InsertPasswordChangedAuditParams{ID: identityID(commit.PasswordAuditID), ActorID: identityID(commit.UserID), OccurredAt: identityTime(commit.ChangedAt), CorrelationID: identityID(commit.CorrelationID), CredentialVersion: commit.NewCredentialVersion, AuthEpoch: commit.NewAuthEpoch}); err != nil {
			return err
		}
		summary, _ := json.Marshal(struct {
			AuthEpoch         int64  `json:"auth_epoch"`
			CredentialVersion int64  `json:"credential_version"`
			Reason            string `json:"reason"`
		}{AuthEpoch: commit.NewAuthEpoch, CredentialVersion: commit.NewCredentialVersion, Reason: "PASSWORD_CHANGED"})
		return queries.InsertLoginAudit(txCtx, identitysql.InsertLoginAuditParams{ID: identityID(commit.LoginAuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.ReplacementSessionID), OccurredAt: identityTime(commit.ChangedAt), CorrelationID: identityID(commit.CorrelationID), ChangeSummary: summary})
	})
}
