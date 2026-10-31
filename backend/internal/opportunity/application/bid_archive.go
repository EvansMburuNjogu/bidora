package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrBidArchiveConflict = errors.New("bid_archive_conflict")
var ErrBidArchiveNotFound = errors.New("bid_archive_not_found")

type ArchiveBidInput struct {
	Subject                  WorkflowSubject
	BidID, CorrelationID     ID
	ExpectedWorkspaceVersion int64
	Reason, IdempotencyKey   string
}
type BidArchive struct {
	ID, BidID, ArchivedBy ID
	WorkspaceVersion      int64
	Reason                string
	PolicyVersion         int32
	ArchivedAt            time.Time
}
type BidArchiveResult struct {
	Archive BidArchive
	Replay  bool
}
type BidArchiveCommit struct {
	CommandID, ArchiveID, AuditID, OutboxID, BidID, CorrelationID ID
	ExpectedWorkspaceVersion                                      int64
	Reason                                                        string
	KeyHash, RequestHash                                          [32]byte
	OccurredAt                                                    time.Time
}
type BidArchiveStore interface {
	FindBidArchiveReplay(context.Context, WorkflowAuthority, ID, [32]byte, [32]byte) (BidArchiveResult, bool, error)
	CommitBidArchive(context.Context, WorkflowAuthority, BidArchiveCommit) (BidArchiveResult, error)
	GetBidArchive(context.Context, WorkflowAuthority, ID) (BidArchive, error)
}
type BidArchiveAuthorizer interface {
	AuthorizeBidArchive(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}

func terminalReportedOutcome(value string) bool {
	switch value {
	case "WON", "LOST", "DISQUALIFIED", "CANCELLED", "WITHDRAWN":
		return true
	default:
		return false
	}
}

type BidArchiveService struct {
	auth  BidArchiveAuthorizer
	store BidArchiveStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewBidArchiveService(a BidArchiveAuthorizer, s BidArchiveStore, ids CaptureIDGenerator, c CaptureClock) (*BidArchiveService, error) {
	if a == nil || s == nil || ids == nil || c == nil {
		return nil, errors.New("bid_archive_service_invalid")
	}
	return &BidArchiveService{a, s, ids, c}, nil
}
func (s *BidArchiveService) Archive(ctx context.Context, in ArchiveBidInput) (BidArchiveResult, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedWorkspaceVersion < 1 || !validRequirementText(in.Reason, 5000) || in.IdempotencyKey == "" {
		return BidArchiveResult{}, safe("bid_archive_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidArchive(ctx, in.Subject)
	if e != nil {
		return BidArchiveResult{}, safe("bid_archive_not_authorized", e)
	}
	raw, _ := json.Marshal(struct {
		Bid     ID
		Version int64
		Reason  string
	}{in.BidID, in.ExpectedWorkspaceVersion, in.Reason})
	kh := sha256.Sum256([]byte("bid-archive:v1:" + in.IdempotencyKey))
	rh := sha256.Sum256(raw)
	if v, found, e := s.store.FindBidArchiveReplay(ctx, a, in.BidID, kh, rh); errors.Is(e, ErrBidArchiveConflict) {
		return BidArchiveResult{}, safe("bid_archive_conflict", e)
	} else if e != nil {
		return BidArchiveResult{}, safe("bid_archive_unavailable", e)
	} else if found {
		return v, nil
	}
	ids, e := responseDraftIDs(s.ids, 4)
	if e != nil {
		return BidArchiveResult{}, safe("bid_archive_unavailable", e)
	}
	v, e := s.store.CommitBidArchive(ctx, a, BidArchiveCommit{CommandID: ids[0], ArchiveID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, CorrelationID: in.CorrelationID, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, Reason: in.Reason, KeyHash: kh, RequestHash: rh, OccurredAt: s.clock.Now().UTC()})
	if errors.Is(e, ErrBidArchiveConflict) {
		return BidArchiveResult{}, safe("bid_archive_conflict", e)
	}
	if e != nil {
		return BidArchiveResult{}, safe("bid_archive_unavailable", e)
	}
	return v, nil
}
func (s *BidArchiveService) Get(ctx context.Context, sub WorkflowSubject, bid ID) (BidArchive, error) {
	if ctx == nil || !validWorkflowSubject(sub) || bid.IsZero() {
		return BidArchive{}, safe("bid_archive_read_invalid", nil)
	}
	a, e := s.auth.AuthorizeBidRead(ctx, sub)
	if e != nil {
		return BidArchive{}, safe("bid_archive_not_authorized", e)
	}
	v, e := s.store.GetBidArchive(ctx, a, bid)
	if errors.Is(e, ErrBidArchiveNotFound) {
		return v, safe("bid_archive_not_found", e)
	}
	if e != nil {
		return v, safe("bid_archive_unavailable", e)
	}
	return v, nil
}
