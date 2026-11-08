package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/platformsettings/domain"
)

type RedactedSMTP struct {
	Username                                                                                   *string // Active authorized form only; never a password or secret reference.
	Host, SenderName, SenderAddress                                                            string
	ReplyToAddress                                                                             *string
	Port, ConnectTimeoutSeconds, CommandTimeoutSeconds, DeliveryTimeoutSeconds, MaxConcurrency int32
}
type RedactedOpenAI struct {
	Enabled                        bool
	Model                          string
	TimeoutSeconds, MaxConcurrency int32
}
type RedactedPesaPal struct {
	CredentialsConfigured     bool
	Enabled                   bool
	Environment, MerchantName string
	NotificationID            domain.ID
}

type RedactedVersion struct {
	PendingChangeID                     domain.ID
	PesaPal                             *RedactedPesaPal
	ID, LatestTestID                    domain.ID
	Kind                                domain.Kind
	Sequence                            int64
	State                               domain.VersionState
	Version                             int64
	SMTP                                *RedactedSMTP
	OpenAI                              *RedactedOpenAI
	TestStatus, TestCode                string
	CreatedAt, UpdatedAt                time.Time
	TestedAt, ActivatedAt, SupersededAt *time.Time
}
type RedactedChange struct {
	ID, TargetVersionID, TargetTestID, ExpectedActiveVersionID, MakerUserID, CheckerUserID domain.ID
	Kind                                                                                   domain.Kind
	Action                                                                                 domain.Action
	State                                                                                  domain.ChangeState
	TargetPriorState                                                                       domain.VersionState
	Version                                                                                int64
	CreatedAt, ExpiresAt                                                                   time.Time
	DecidedAt                                                                              *time.Time
}
type VersionPage struct {
	Versions     []RedactedVersion
	NextSequence *int64
}

type ReadTx interface {
	Authorize(context.Context, Subject, time.Time) (Authority, error)
	GetActiveVersion(context.Context, domain.Kind) (RedactedVersion, bool, error)
	ListVersions(context.Context, domain.Kind, *int64, int32) ([]RedactedVersion, error)
	GetChangeView(context.Context, domain.ID) (RedactedChange, bool, error)
}
type ReadUnitOfWork interface {
	WithinSettingsRead(context.Context, func(context.Context, ReadTx) error) error
}
type SMTPUsernameReader interface {
	SMTPUsername(context.Context, domain.ID) (string, error)
}

type ReadService struct {
	smtpUsernames SMTPUsernameReader
	uow           ReadUnitOfWork
	clock         Clock
}

func NewReadService(uow ReadUnitOfWork, clock Clock) (*ReadService, error) {
	if uow == nil || clock == nil {
		return nil, errors.New("platform_setting_read_service_invalid")
	}
	return &ReadService{uow: uow, clock: clock}, nil
}

func (s *ReadService) EnableSMTPUsernames(reader SMTPUsernameReader) { s.smtpUsernames = reader }
func (s *ReadService) Active(ctx context.Context, subject Subject, kind domain.Kind) (result RedactedVersion, err error) {
	if !validRead(ctx, s, subject, kind) {
		return result, ErrInvalid
	}
	err = s.withRead(ctx, subject, func(txCtx context.Context, tx ReadTx) error {
		var found bool
		result, found, err = tx.GetActiveVersion(txCtx, kind)
		if err == nil && !found {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return RedactedVersion{}, mapError(err)
	}
	if result.SMTP != nil && s.smtpUsernames != nil {
		username, readErr := s.smtpUsernames.SMTPUsername(ctx, result.ID)
		if readErr != nil {
			return RedactedVersion{}, ErrUnavailable
		}
		result.SMTP.Username = &username
	}
	return result, nil
}
func (s *ReadService) Versions(ctx context.Context, subject Subject, kind domain.Kind, before *int64, limit int32) (result VersionPage, err error) {
	if !validRead(ctx, s, subject, kind) || limit < 1 || limit > 100 || before != nil && *before < 1 {
		return result, ErrInvalid
	}
	err = s.withRead(ctx, subject, func(txCtx context.Context, tx ReadTx) error {
		result.Versions, err = tx.ListVersions(txCtx, kind, before, limit+1)
		if err != nil {
			return err
		}
		if len(result.Versions) > int(limit) {
			result.Versions = result.Versions[:limit]
			next := result.Versions[len(result.Versions)-1].Sequence
			result.NextSequence = &next
		}
		return nil
	})
	if err != nil {
		return VersionPage{}, mapError(err)
	}
	return result, nil
}
func (s *ReadService) Change(ctx context.Context, subject Subject, id domain.ID) (result RedactedChange, err error) {
	if ctx == nil || s == nil || id.IsZero() || !validSubject(subject) {
		return result, ErrInvalid
	}
	err = s.withRead(ctx, subject, func(txCtx context.Context, tx ReadTx) error {
		var found bool
		result, found, err = tx.GetChangeView(txCtx, id)
		if err == nil && !found {
			return ErrNotFound
		}
		if err == nil && result.State == domain.ChangePending && !s.clock.Now().UTC().Before(result.ExpiresAt) {
			result.State = domain.ChangeExpired
		}
		return err
	})
	if err != nil {
		return RedactedChange{}, mapError(err)
	}
	return result, nil
}
func (s *ReadService) withRead(ctx context.Context, subject Subject, read func(context.Context, ReadTx) error) error {
	at := s.clock.Now().UTC()
	return s.uow.WithinSettingsRead(ctx, func(txCtx context.Context, tx ReadTx) error {
		authority, err := tx.Authorize(txCtx, subject, at)
		if err != nil {
			return err
		}
		if !authority.Capabilities["platform.integration.read"] || !recent(authority, at) {
			return ErrForbidden
		}
		return read(txCtx, tx)
	})
}
func validRead(ctx context.Context, service *ReadService, subject Subject, kind domain.Kind) bool {
	return ctx != nil && service != nil && service.uow != nil && service.clock != nil && validSubject(subject) && (kind == domain.SMTP || kind == domain.OpenAI || kind == domain.PesaPal)
}
