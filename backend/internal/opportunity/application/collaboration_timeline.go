package application

import (
	"context"
	"errors"
	"time"
)

var ErrCollaborationTimelineNotFound = errors.New("collaboration_timeline_not_found")

type CollaborationTimelineInput struct {
	Subject WorkflowSubject
	BidID   ID
	Cursor  ReadCursor
	Limit   int32
}
type CollaborationActivity struct {
	ID, ActorID, TargetID, RelatedID  ID
	EventType, TargetType, DetailCode string
	Version                           int64
	OccurredAt                        time.Time
	DueAt                             *time.Time
}
type CollaborationTimelinePage struct {
	Items   []CollaborationActivity
	HasMore bool
}
type CollaborationTimelineStore interface {
	ListCollaborationTimeline(context.Context, WorkflowAuthority, ID, ReadCursor, int32) ([]CollaborationActivity, error)
}
type CollaborationTimelineService struct {
	auth  TechnicalResponseAuthorizer
	store CollaborationTimelineStore
}

func NewCollaborationTimelineService(auth TechnicalResponseAuthorizer, store CollaborationTimelineStore) (*CollaborationTimelineService, error) {
	if auth == nil || store == nil {
		return nil, errors.New("collaboration_timeline_service_invalid")
	}
	return &CollaborationTimelineService{auth: auth, store: store}, nil
}
func (s *CollaborationTimelineService) List(ctx context.Context, in CollaborationTimelineInput) (CollaborationTimelinePage, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.Limit < 1 || in.Limit > 100 || (in.Cursor.CreatedAt.IsZero() != in.Cursor.ID.IsZero()) {
		return CollaborationTimelinePage{}, safe("collaboration_timeline_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, in.Subject)
	if err != nil {
		return CollaborationTimelinePage{}, safe("collaboration_timeline_not_authorized", err)
	}
	rows, err := s.store.ListCollaborationTimeline(ctx, a, in.BidID, in.Cursor, in.Limit+1)
	if errors.Is(err, ErrCollaborationTimelineNotFound) {
		return CollaborationTimelinePage{}, safe("collaboration_timeline_not_found", err)
	}
	if err != nil {
		return CollaborationTimelinePage{}, safe("collaboration_timeline_unavailable", err)
	}
	page := CollaborationTimelinePage{Items: rows}
	if len(rows) > int(in.Limit) {
		page.HasMore = true
		page.Items = rows[:in.Limit]
	}
	return page, nil
}
