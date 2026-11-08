package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/commercial/domain"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"github.com/google/uuid"
)

var ErrPlatformPlanInvalid = errors.New("platform_plan_invalid")
var ErrPlatformPlanForbidden = errors.New("platform_plan_forbidden")
var ErrPlatformPlanConflict = errors.New("platform_plan_conflict")
var ErrPlatformPlanUnavailable = errors.New("platform_plan_unavailable")

type PlanConfig struct {
	Key                                               domain.PlanKey
	Version, HeadVersion                              int64
	MonthlyMinor, AnnualMinor, AdditionalLicenceMinor int64
	BidWorkspaces, IncludedLicences, ConcurrentAIJobs int32
	StorageBytes, DocumentVersions, AICredits         int64
	CreatedBy                                         domain.ID
}

func (v PlanConfig) fieldsValid() bool {
	return (v.Key == domain.Basic || v.Key == domain.Growth || v.Key == domain.Professional) && v.MonthlyMinor > 0 && v.MonthlyMinor <= 1_000_000_000_000 && v.AnnualMinor > 0 && v.AnnualMinor <= 1_000_000_000_000 && v.AdditionalLicenceMinor > 0 && v.AdditionalLicenceMinor <= 1_000_000_000_000 && v.BidWorkspaces > 0 && v.BidWorkspaces <= 1_000_000 && v.IncludedLicences > 0 && v.IncludedLicences <= 1_000_000 && v.ConcurrentAIJobs > 0 && v.ConcurrentAIJobs <= 100_000 && v.StorageBytes > 0 && v.StorageBytes <= 1_000_000_000_000_000 && v.DocumentVersions > 0 && v.DocumentVersions <= 1_000_000_000 && v.AICredits > 0 && v.AICredits <= 1_000_000_000
}
func (v PlanConfig) Valid() bool { return v.Version > 0 && v.fieldsValid() }
func ApplyPublishedPlan(v PlanConfig) (domain.Plan, error) {
	if !v.Valid() {
		return domain.Plan{}, ErrPlatformPlanUnavailable
	}
	plan, err := domain.FindPlan(v.Key)
	if err != nil {
		return domain.Plan{}, ErrPlatformPlanUnavailable
	}
	if v.Version > 1 {
		plan.CatalogueVersion = fmt.Sprintf("PHASE1_V1-%s-v%d", v.Key, v.Version)
	}
	plan.MonthlyBasePrice = &domain.Money{Currency: domain.CurrencyKES, MinorUnits: v.MonthlyMinor}
	plan.AnnualBasePrice = &domain.Money{Currency: domain.CurrencyKES, MinorUnits: v.AnnualMinor}
	plan.AdditionalLicenceMonthlyPrice = &domain.Money{Currency: domain.CurrencyKES, MinorUnits: v.AdditionalLicenceMinor}
	plan.BidWorkspacesPerPeriod = &v.BidWorkspaces
	plan.IncludedLicences = &v.IncludedLicences
	plan.StorageLimitBytes = &v.StorageBytes
	plan.DocumentVersionLimit = &v.DocumentVersions
	plan.AICreditLimit = &v.AICredits
	plan.ConcurrentAIJobLimit = &v.ConcurrentAIJobs
	return plan, nil
}

type PlanHead struct{ ActiveVersion, NextVersion, HeadVersion int64 }
type PlanCommandRecord struct {
	RequestHash          [32]byte
	Key                  domain.PlanKey
	Version, HeadVersion int64
}
type PlanSubject struct {
	UserID, SessionID, CorrelationID domain.ID
	ContextGeneration                int64
}
type PlanChange struct {
	Subject                             PlanSubject
	Operation                           string
	Config                              PlanConfig
	PublishVersion, ExpectedHeadVersion int64
	IdempotencyKey                      string
}
type PlanReceipt struct {
	Key                  domain.PlanKey
	Version, HeadVersion int64
	Published, Replay    bool
}
type PlanManagementItem struct {
	Published PlanConfig
	Draft     *PlanConfig
	Head      PlanHead
}
type PlanTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ListPublished(context.Context) ([]PlanConfig, error)
	LockPlanHead(context.Context, domain.PlanKey) (PlanHead, error)
	GetPlanVersion(context.Context, domain.PlanKey, int64) (PlanConfig, error)
	GetPlanCommand(context.Context, domain.ID, string, [32]byte) (PlanCommandRecord, bool, error)
	InsertPlanVersion(context.Context, PlanConfig, domain.ID, time.Time) error
	AdvancePlanDraft(context.Context, domain.PlanKey, int64, int64, time.Time) (bool, error)
	PublishPlan(context.Context, domain.PlanKey, int64, int64, time.Time) (bool, error)
	AppendPlanCommand(context.Context, domain.ID, string, [32]byte, [32]byte, PlanReceipt, time.Time) error
	AppendPlanEvidence(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, domain.PlanKey, string, int64, time.Time) error
	AppendPlanRead(context.Context, domain.ID, domain.ID, domain.ID, time.Time) error
}
type PlanUnitOfWork interface {
	WithinPlan(context.Context, func(context.Context, PlanTx) error) error
}
type PublishedPlanReader interface {
	PublishedPlans(context.Context) ([]PlanConfig, error)
}
type PlanIDs interface{ NewID() (domain.ID, error) }
type PlanClock interface{ Now() time.Time }
type PlanService struct {
	uow   PlanUnitOfWork
	ids   PlanIDs
	clock PlanClock
}

func NewPlanService(uow PlanUnitOfWork, ids PlanIDs, clock PlanClock) (*PlanService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrPlatformPlanUnavailable
	}
	return &PlanService{uow, ids, clock}, nil
}
func (s *PlanService) ListManagement(ctx context.Context, subject PlanSubject) (out []PlanManagementItem, err error) {
	if s == nil || ctx == nil || subject.UserID.IsZero() || subject.SessionID.IsZero() || subject.CorrelationID.IsZero() || subject.ContextGeneration < 1 {
		return nil, ErrPlatformPlanInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return nil, ErrPlatformPlanUnavailable
	}
	err = s.uow.WithinPlan(ctx, func(c context.Context, tx PlanTx) error {
		facts, x := tx.ResolvePlatformAuthority(c, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.UserID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
		if errors.Is(x, platformapp.ErrAuthorityNotFound) {
			return ErrPlatformPlanForbidden
		}
		if x != nil {
			return x
		}
		if !facts.Grants("platform.plan.read") || facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrPlatformPlanForbidden
		}
		versions, x := tx.ListPublished(c)
		if x != nil {
			return x
		}
		if len(versions) != 3 {
			return ErrPlatformPlanUnavailable
		}
		seen := map[domain.PlanKey]bool{}
		for _, v := range versions {
			if !v.Valid() || seen[v.Key] {
				return ErrPlatformPlanUnavailable
			}
			seen[v.Key] = true
			head, e := tx.LockPlanHead(c, v.Key)
			if e != nil {
				return e
			}
			if head.ActiveVersion != v.Version || head.HeadVersion != v.HeadVersion || head.NextVersion <= head.ActiveVersion {
				return ErrPlatformPlanUnavailable
			}
			item := PlanManagementItem{Published: v, Head: head}
			if head.NextVersion > head.ActiveVersion+1 {
				draft, e := tx.GetPlanVersion(c, v.Key, head.NextVersion-1)
				if e != nil {
					return e
				}
				if !draft.Valid() {
					return ErrPlatformPlanUnavailable
				}
				item.Draft = &draft
			}
			out = append(out, item)
		}
		if !seen[domain.Basic] || !seen[domain.Growth] || !seen[domain.Professional] {
			return ErrPlatformPlanUnavailable
		}
		audit, e := s.ids.NewID()
		if e != nil || audit.IsZero() {
			return ErrPlatformPlanUnavailable
		}
		return tx.AppendPlanRead(c, audit, subject.UserID, subject.CorrelationID, at)
	})
	if err != nil {
		if errors.Is(err, ErrPlatformPlanForbidden) {
			return nil, err
		}
		return nil, ErrPlatformPlanUnavailable
	}
	return out, nil
}
func (s *PlanService) Change(ctx context.Context, in PlanChange) (out PlanReceipt, err error) {
	if s == nil || ctx == nil || in.Subject.UserID.IsZero() || in.Subject.SessionID.IsZero() || in.Subject.CorrelationID.IsZero() || in.Subject.ContextGeneration < 1 || in.ExpectedHeadVersion < 1 || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 || in.Operation != "DRAFT" && in.Operation != "PUBLISH" || in.Operation == "DRAFT" && (!in.Config.fieldsValid() || in.Config.Version != 0 || !in.Config.CreatedBy.IsZero() || in.PublishVersion != 0) || in.Operation == "PUBLISH" && (in.PublishVersion < 2 || in.Config.Key != domain.Basic && in.Config.Key != domain.Growth && in.Config.Key != domain.Professional) {
		return out, ErrPlatformPlanInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrPlatformPlanUnavailable
	}
	parsedKey, e := uuid.Parse(in.IdempotencyKey)
	if e != nil || parsedKey == uuid.Nil || parsedKey.String() != in.IdempotencyKey {
		return out, ErrPlatformPlanInvalid
	}
	keyHash := sha256.Sum256([]byte("platform-plan:" + in.Operation + ":" + in.IdempotencyKey))
	encoded, _ := json.Marshal(struct {
		Subject                             domain.ID
		Generation                          int64
		Operation                           string
		Config                              PlanConfig
		PublishVersion, ExpectedHeadVersion int64
	}{in.Subject.UserID, in.Subject.ContextGeneration, in.Operation, in.Config, in.PublishVersion, in.ExpectedHeadVersion})
	hash := sha256.Sum256(append([]byte("platform-plan:v1:"), encoded...))
	err = s.uow.WithinPlan(ctx, func(c context.Context, tx PlanTx) error {
		facts, x := tx.ResolvePlatformAuthority(c, platformapp.AuthorityQuery{UserID: platformapp.ID(in.Subject.UserID), SessionID: platformapp.ID(in.Subject.SessionID), ObservedAt: at})
		if errors.Is(x, platformapp.ErrAuthorityNotFound) {
			return ErrPlatformPlanForbidden
		}
		if x != nil {
			return x
		}
		capability := "platform.plan.manage"
		if in.Operation == "PUBLISH" {
			capability = "platform.plan.publish"
		}
		if !facts.Grants(capability) || facts.UserID != platformapp.ID(in.Subject.UserID) || facts.ContextGeneration != in.Subject.ContextGeneration || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrPlatformPlanForbidden
		}
		old, found, x := tx.GetPlanCommand(c, in.Subject.UserID, in.Operation, keyHash)
		if x != nil {
			return x
		}
		if found {
			if old.RequestHash != hash || old.Key != in.Config.Key {
				return ErrPlatformPlanConflict
			}
			out = PlanReceipt{Key: old.Key, Version: old.Version, HeadVersion: old.HeadVersion, Published: in.Operation == "PUBLISH", Replay: true}
			return nil
		}
		head, x := tx.LockPlanHead(c, in.Config.Key)
		if x != nil {
			return x
		}
		if head.HeadVersion != in.ExpectedHeadVersion || head.ActiveVersion < 1 || head.NextVersion <= head.ActiveVersion {
			return ErrPlatformPlanConflict
		}
		version := head.NextVersion
		if in.Operation == "DRAFT" {
			if version > 1000000 {
				return ErrPlatformPlanConflict
			}
			config := in.Config
			config.Version = version
			config.CreatedBy = in.Subject.UserID
			if x = tx.InsertPlanVersion(c, config, in.Subject.UserID, at); x != nil {
				return x
			}
			updated, e := tx.AdvancePlanDraft(c, config.Key, head.HeadVersion, version, at)
			if e != nil {
				return e
			}
			if !updated {
				return ErrPlatformPlanConflict
			}
		} else {
			version = in.PublishVersion
			if version <= head.ActiveVersion || version >= head.NextVersion {
				return ErrPlatformPlanConflict
			}
			target, e := tx.GetPlanVersion(c, in.Config.Key, version)
			if e != nil {
				return e
			}
			if !target.Valid() || target.CreatedBy.IsZero() || target.CreatedBy == in.Subject.UserID {
				return ErrPlatformPlanConflict
			}
			updated, e := tx.PublishPlan(c, in.Config.Key, head.HeadVersion, version, at)
			if e != nil {
				return e
			}
			if !updated {
				return ErrPlatformPlanConflict
			}
		}
		out = PlanReceipt{Key: in.Config.Key, Version: version, HeadVersion: head.HeadVersion + 1, Published: in.Operation == "PUBLISH"}
		if x = tx.AppendPlanCommand(c, in.Subject.UserID, in.Operation, keyHash, hash, out, at); x != nil {
			return x
		}
		audit, e := s.ids.NewID()
		if e != nil || audit.IsZero() {
			return ErrPlatformPlanUnavailable
		}
		outbox, e := s.ids.NewID()
		if e != nil || outbox.IsZero() {
			return ErrPlatformPlanUnavailable
		}
		return tx.AppendPlanEvidence(c, audit, outbox, in.Subject.UserID, in.Subject.CorrelationID, in.Config.Key, in.Operation, out.HeadVersion, at)
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPlatformPlanForbidden), errors.Is(err, ErrPlatformPlanInvalid), errors.Is(err, ErrPlatformPlanConflict):
			return PlanReceipt{}, err
		default:
			return PlanReceipt{}, ErrPlatformPlanUnavailable
		}
	}
	return out, nil
}
