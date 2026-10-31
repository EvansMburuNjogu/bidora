package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type PendingInvitation struct {
	InvitationID         ID
	Email                string
	Role                 TenderpreneurRole
	CreatedAt, ExpiresAt time.Time
}
type InvitationCursor struct {
	CreatedAt    time.Time
	InvitationID ID
}
type InvitationDirectoryQuery struct {
	ActorID, TenantID ID
	ContextGeneration int64
	Limit             int
	Cursor            InvitationCursor
}
type InvitationDirectoryRead struct {
	ActorID, TenantID ID
	ContextGeneration int64
	ObservedAt        time.Time
	MaxRows           int32
	Cursor            InvitationCursor
}
type InvitationPage struct {
	Invitations []PendingInvitation
	HasMore     bool
}

func (PendingInvitation) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED pending invitation]")
}
func (InvitationDirectoryQuery) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation directory query]")
}

type InvitationDirectoryReader interface {
	ListPending(context.Context, InvitationDirectoryRead) ([]PendingInvitation, error)
}
type InvitationDirectory struct {
	reader InvitationDirectoryReader
	clock  InvitationClock
}

func NewInvitationDirectory(reader InvitationDirectoryReader, clock InvitationClock) (*InvitationDirectory, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("invitation_directory_invalid")
	}
	return &InvitationDirectory{reader, clock}, nil
}
func (d *InvitationDirectory) List(ctx context.Context, query InvitationDirectoryQuery) (InvitationPage, error) {
	if ctx == nil || query.ActorID.IsZero() || query.TenantID.IsZero() || query.ContextGeneration < 1 || query.Limit < 1 || query.Limit > 100 || (query.Cursor.CreatedAt.IsZero() != query.Cursor.InvitationID.IsZero()) {
		return InvitationPage{}, safe("invitation_directory_invalid", nil)
	}
	if d == nil || d.reader == nil || d.clock == nil {
		return InvitationPage{}, safe("invitation_directory_unavailable", nil)
	}
	now := d.clock.Now().UTC()
	if now.IsZero() {
		return InvitationPage{}, safe("invitation_directory_unavailable", nil)
	}
	rows, err := d.reader.ListPending(ctx, InvitationDirectoryRead{ActorID: query.ActorID, TenantID: query.TenantID, ContextGeneration: query.ContextGeneration, ObservedAt: now, MaxRows: int32(query.Limit + 1), Cursor: query.Cursor})
	if errors.Is(err, ErrMembershipManagementNotAuthorized) {
		return InvitationPage{}, safe("invitation_directory_not_authorized", nil)
	}
	if err != nil {
		return InvitationPage{}, safe("invitation_directory_unavailable", err)
	}
	for _, row := range rows {
		if _, err := ParseTenderpreneurRole(string(row.Role)); err != nil {
			return InvitationPage{}, safe("invitation_directory_unavailable", err)
		}
	}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		return InvitationPage{Invitations: rows, HasMore: true}, nil
	}
	return InvitationPage{Invitations: rows, HasMore: false}, nil
}
