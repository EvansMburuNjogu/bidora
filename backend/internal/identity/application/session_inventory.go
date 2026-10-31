package application

import (
	"context"
	"errors"
	"time"
)

var ErrSessionInventoryLimit = errors.New("session_inventory_limit")

type SessionInventoryItem struct {
	SessionID, RotatedFromSessionID                         ID
	CreatedAt, LastSeenAt, IdleExpiresAt, AbsoluteExpiresAt time.Time
	Current                                                 bool
}
type SessionInventory struct {
	Sessions                                                                                                       []SessionInventoryItem
	ActiveSessionCount, OtherSessionCount                                                                          int
	CurrentSessionLogoutAvailable, OtherSessionRevocationAvailable, RevokeAllAvailable, RevocationPolicyConfigured bool
	Blockers                                                                                                       []string
	ObservedAt                                                                                                     time.Time
}
type SessionInventoryReader interface {
	ListActiveUserSessions(context.Context, ID, time.Time) ([]SessionInventoryItem, error)
}
type SessionInventoryService struct {
	reader SessionInventoryReader
	clock  SessionClock
}

func NewSessionInventoryService(reader SessionInventoryReader, clock SessionClock) (*SessionInventoryService, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("session_inventory_service_invalid")
	}
	return &SessionInventoryService{reader: reader, clock: clock}, nil
}
func (s *SessionInventoryService) Get(ctx context.Context, principal SessionPrincipal) (SessionInventory, error) {
	if ctx == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || principal.AuthEpoch < 1 {
		return SessionInventory{}, safe("session_inventory_invalid", nil)
	}
	observed := s.clock.Now().UTC()
	rows, err := s.reader.ListActiveUserSessions(ctx, principal.UserID, observed)
	if errors.Is(err, ErrSessionInventoryLimit) {
		return SessionInventory{}, safe("session_inventory_limit", err)
	}
	if err != nil {
		return SessionInventory{}, safe("session_inventory_unavailable", err)
	}
	result := SessionInventory{Sessions: rows, ActiveSessionCount: len(rows), CurrentSessionLogoutAvailable: true, RevocationPolicyConfigured: true, RevokeAllAvailable: true, ObservedAt: observed}
	currentFound := false
	for i := range result.Sessions {
		result.Sessions[i].Current = result.Sessions[i].SessionID == principal.SessionID
		if result.Sessions[i].Current {
			currentFound = true
		} else {
			result.OtherSessionCount++
		}
	}
	if !currentFound {
		return SessionInventory{}, safe("session_inventory_unavailable", nil)
	}
	result.OtherSessionRevocationAvailable = result.OtherSessionCount > 0
	return result, nil
}
