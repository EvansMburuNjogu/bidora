package persistence

import (
	"bytes"
	"context"
	"errors"

	"bidora/backend/internal/identity/accounttype"
	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type accountTypeRunner interface {
	readOnlyRunner
	writeRunner
}

type AccountTypeStore struct{ runner accountTypeRunner }

func NewAccountTypeStore(runner accountTypeRunner) (*AccountTypeStore, error) {
	if runner == nil {
		return nil, errors.New("account_type_store_invalid")
	}
	return &AccountTypeStore{runner: runner}, nil
}

func (s *AccountTypeStore) GetAccountType(ctx context.Context, userID identityapp.ID) (result accounttype.Type, err error) {
	if ctx == nil || userID.IsZero() || s == nil || s.runner == nil {
		return "", errors.New("account_type_read_invalid")
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		persisted, queryErr := identitysql.New(db).GetUserAccountType(txCtx, identityID(userID))
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return identityapp.ErrAccountTypeNotFound
		}
		if queryErr != nil {
			return queryErr
		}
		result, queryErr = accounttype.ParsePersisted(persisted)
		return queryErr
	})
	return result, err
}

func (s *AccountTypeStore) SelectAccountType(ctx context.Context, commit identityapp.AccountTypeSelectionCommit) (result identityapp.AccountTypeSelectionResult, err error) {
	if ctx == nil || s == nil || s.runner == nil || commit.CommandID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.UserID.IsZero() || commit.CorrelationID.IsZero() || commit.KeyHash == ([32]byte{}) || commit.RequestHash == ([32]byte{}) || commit.OccurredAt.IsZero() {
		return result, errors.New("account_type_commit_invalid")
	}
	if _, parseErr := accounttype.ParseSelfSelection(string(commit.AccountType)); parseErr != nil {
		return result, errors.New("account_type_commit_invalid")
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		queries := identitysql.New(db)
		persisted, queryErr := queries.LockUserAccountTypeForSelection(txCtx, identityID(commit.UserID))
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return identityapp.ErrAccountTypeNotFound
		}
		if queryErr != nil {
			return queryErr
		}

		prior, queryErr := queries.GetAccountTypeSelectionCommand(txCtx, identitysql.GetAccountTypeSelectionCommandParams{
			ActorUserID: identityID(commit.UserID), KeyHash: commit.KeyHash[:],
		})
		if queryErr == nil {
			if !bytes.Equal(prior.RequestHash, commit.RequestHash[:]) || prior.SelectedAccountType != string(commit.AccountType) || prior.ResponseStatus != 204 {
				return identityapp.ErrAccountTypeConflict
			}
			result.Replay = true
			return nil
		}
		if !errors.Is(queryErr, pgx.ErrNoRows) {
			return queryErr
		}
		current, parseErr := accounttype.ParsePersisted(persisted)
		if parseErr != nil {
			return parseErr
		}
		if current != accounttype.Unassigned {
			return identityapp.ErrAccountTypeAlreadySelected
		}

		claimed, queryErr := queries.ClaimAccountTypeSelectionCommand(txCtx, identitysql.ClaimAccountTypeSelectionCommandParams{
			ID: identityID(commit.CommandID), ActorUserID: identityID(commit.UserID), KeyHash: commit.KeyHash[:],
			RequestHash: commit.RequestHash[:], SelectedAccountType: string(commit.AccountType), CompletedAt: identityTime(commit.OccurredAt),
		})
		if queryErr != nil {
			return queryErr
		}
		if claimed != 1 {
			return identityapp.ErrAccountTypeConflict
		}
		selected := string(commit.AccountType)
		changed, queryErr := queries.SelectUserAccountType(txCtx, identitysql.SelectUserAccountTypeParams{
			AccountType: &selected, SelectedAt: identityTime(commit.OccurredAt), UserID: identityID(commit.UserID),
		})
		if queryErr != nil {
			return queryErr
		}
		if changed != 1 {
			return identityapp.ErrAccountTypeAlreadySelected
		}
		if queryErr = queries.InsertAccountTypeSelectedAudit(txCtx, identitysql.InsertAccountTypeSelectedAuditParams{
			ID: identityID(commit.AuditID), ActorID: identityID(commit.UserID), OccurredAt: identityTime(commit.OccurredAt),
			CorrelationID: identityID(commit.CorrelationID), AccountType: selected,
		}); queryErr != nil {
			return queryErr
		}
		return queries.InsertAccountTypeSelectedOutbox(txCtx, identitysql.InsertAccountTypeSelectedOutboxParams{
			ID: identityID(commit.OutboxID), ActorID: identityID(commit.UserID), AccountType: selected, OccurredAt: identityTime(commit.OccurredAt),
		})
	})
	return result, err
}
