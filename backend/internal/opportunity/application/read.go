package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ReadCursor struct {
	CreatedAt time.Time
	ID        ID
}
type ReadQuery struct {
	Subject WorkflowSubject
	Limit   int
	Cursor  ReadCursor
}
type BidReadQuery struct {
	ReadQuery
	State        string
	ArchiveState string
	OwnerID      ID
	FilterOwner  bool
}
type OpportunitySummary struct {
	ID, SourceVersionID  ID
	Version              int64
	Title                string
	BuyerName            *string
	ClosesAt             *time.Time
	SourceTimezone       *string
	VerificationState    string
	CreatedAt, UpdatedAt time.Time
}
type OpportunityDetail struct {
	OpportunitySummary
	Revision                int32
	SourceURI               *string
	SourceDocumentVersionID ID
	HasSourceDocument       bool
	SourceDateText          *string
	VerifiedBy              ID
	VerifiedAt              *time.Time
	PossibleDuplicateIDs    []ID
	DuplicatesTruncated     bool
}
type BidSummary struct {
	ID, OpportunityID, SourceVersionID, OwnerID ID
	State                                       string
	Version                                     int64
	Title                                       string
	BuyerName                                   *string
	ClosesAt                                    *time.Time
	SourceTimezone                              *string
	CreatedAt, UpdatedAt                        time.Time
	ArchivedAt                                  *time.Time
}
type DecisionHistoryItem struct {
	ID, ActorID         ID
	Sequence            int32
	WorkspaceVersion    int64
	Decision, Rationale string
	DecidedAt           time.Time
}
type BidDetail struct {
	BidSummary
	CreatedBy, ArchivedBy   ID
	ArchiveReason           *string
	ArchivePolicyVersion    *int32
	SourceRevision          int32
	SourceURI               *string
	SourceDocumentVersionID ID
	HasSourceDocument       bool
	SourceDateText          *string
	VerificationState       string
	Decisions               []DecisionHistoryItem
}
type OpportunityPage struct {
	Items   []OpportunitySummary
	HasMore bool
}
type BidPage struct {
	Items   []BidSummary
	HasMore bool
}

func (ReadQuery) Format(s fmt.State, verb rune)    { fmt.Fprint(s, "[REDACTED opportunity read query]") }
func (BidReadQuery) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED bid read query]") }

type WorkflowReadStore interface {
	ListOpportunities(context.Context, WorkflowAuthority, ReadCursor, int32) ([]OpportunitySummary, error)
	GetOpportunity(context.Context, WorkflowAuthority, ID) (OpportunityDetail, error)
	ListBids(context.Context, WorkflowAuthority, BidReadQuery, int32) ([]BidSummary, error)
	GetBid(context.Context, WorkflowAuthority, ID) (BidDetail, error)
}
type WorkflowReadService struct {
	auth  WorkflowAuthorizer
	store WorkflowReadStore
}

func NewWorkflowReadService(auth WorkflowAuthorizer, store WorkflowReadStore) (*WorkflowReadService, error) {
	if auth == nil || store == nil {
		return nil, errors.New("workflow_read_service_invalid")
	}
	return &WorkflowReadService{auth, store}, nil
}
func (s *WorkflowReadService) ListOpportunities(ctx context.Context, q ReadQuery) (OpportunityPage, error) {
	if !validRead(ctx, q.Subject, q.Limit, q.Cursor) {
		return OpportunityPage{}, safe("opportunity_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeOpportunityRead(ctx, q.Subject)
	if err != nil {
		return OpportunityPage{}, safe("opportunity_read_not_authorized", err)
	}
	rows, err := s.store.ListOpportunities(ctx, authority, q.Cursor, int32(q.Limit+1))
	if err != nil {
		return OpportunityPage{}, safe("opportunity_read_unavailable", err)
	}
	if len(rows) > q.Limit {
		return OpportunityPage{Items: rows[:q.Limit], HasMore: true}, nil
	}
	return OpportunityPage{Items: rows}, nil
}
func (s *WorkflowReadService) GetOpportunity(ctx context.Context, subject WorkflowSubject, id ID) (OpportunityDetail, error) {
	if ctx == nil || !validWorkflowSubject(subject) || id.IsZero() {
		return OpportunityDetail{}, safe("opportunity_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeOpportunityRead(ctx, subject)
	if err != nil {
		return OpportunityDetail{}, safe("opportunity_read_not_authorized", err)
	}
	result, err := s.store.GetOpportunity(ctx, authority, id)
	if err != nil {
		return OpportunityDetail{}, safe("opportunity_not_found", err)
	}
	return result, nil
}
func (s *WorkflowReadService) ListBids(ctx context.Context, q BidReadQuery) (BidPage, error) {
	if !validRead(ctx, q.Subject, q.Limit, q.Cursor) || (q.State != "" && q.State != "DECISION_PENDING" && q.State != "BID" && q.State != "NO_BID" && q.State != "SUBMITTED") || (q.ArchiveState != "ACTIVE" && q.ArchiveState != "ARCHIVED") || (q.FilterOwner && q.OwnerID.IsZero()) {
		return BidPage{}, safe("bid_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, q.Subject)
	if err != nil {
		return BidPage{}, safe("bid_read_not_authorized", err)
	}
	rows, err := s.store.ListBids(ctx, authority, q, int32(q.Limit+1))
	if err != nil {
		return BidPage{}, safe("bid_read_unavailable", err)
	}
	if len(rows) > q.Limit {
		return BidPage{Items: rows[:q.Limit], HasMore: true}, nil
	}
	return BidPage{Items: rows}, nil
}
func (s *WorkflowReadService) GetBid(ctx context.Context, subject WorkflowSubject, id ID) (BidDetail, error) {
	if ctx == nil || !validWorkflowSubject(subject) || id.IsZero() {
		return BidDetail{}, safe("bid_read_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return BidDetail{}, safe("bid_read_not_authorized", err)
	}
	result, err := s.store.GetBid(ctx, authority, id)
	if err != nil {
		return BidDetail{}, safe("bid_not_found", err)
	}
	return result, nil
}
func validRead(ctx context.Context, subject WorkflowSubject, limit int, cursor ReadCursor) bool {
	return ctx != nil && validWorkflowSubject(subject) && limit >= 1 && limit <= 100 && (cursor.CreatedAt.IsZero() == cursor.ID.IsZero())
}
