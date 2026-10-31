package application

import (
	"context"
	"errors"
	"fmt"
)

type MembershipSummary struct {
	MembershipID, TenantID ID
	TenantType, TenantName string
	ContextGeneration      int64
	Capabilities           []string
}

func (MembershipSummary) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED membership summary]")
}

type ManagedContributor struct {
	MembershipID      ID
	Email             string
	ContextGeneration int64
}
type ManagedContributorQuery struct {
	ActorID, TenantID ID
	ContextGeneration int64
}

func (ManagedContributor) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED managed contributor]")
}

var ErrMembershipManagementNotAuthorized = errors.New("membership_management_not_authorized")

type MembershipDirectoryReader interface {
	ListActiveMemberships(context.Context, ID) ([]MembershipSummary, error)
	ListManagedContributors(context.Context, ManagedContributorQuery) ([]ManagedContributor, error)
}
type MembershipDirectory struct{ reader MembershipDirectoryReader }

func NewMembershipDirectory(reader MembershipDirectoryReader) (*MembershipDirectory, error) {
	if reader == nil {
		return nil, errors.New("membership_directory_invalid")
	}
	return &MembershipDirectory{reader}, nil
}
func (d *MembershipDirectory) List(ctx context.Context, userID ID) ([]MembershipSummary, error) {
	if ctx == nil || userID.IsZero() {
		return nil, safe("membership_directory_invalid", nil)
	}
	if d == nil || d.reader == nil {
		return nil, safe("membership_directory_unavailable", nil)
	}
	rows, err := d.reader.ListActiveMemberships(ctx, userID)
	if err != nil {
		return nil, safe("membership_directory_unavailable", err)
	}
	result := make([]MembershipSummary, len(rows))
	for index, row := range rows {
		if row.MembershipID.IsZero() || row.TenantID.IsZero() || row.TenantType != "TENDERPRENEUR" || row.TenantName == "" || row.ContextGeneration < 1 || row.Capabilities == nil {
			return nil, safe("membership_directory_unavailable", nil)
		}
		result[index] = MembershipSummary{MembershipID: row.MembershipID, TenantID: row.TenantID, TenantType: row.TenantType, TenantName: row.TenantName, ContextGeneration: row.ContextGeneration, Capabilities: append([]string(nil), row.Capabilities...)}
	}
	return result, nil
}
func (d *MembershipDirectory) ListManaged(ctx context.Context, query ManagedContributorQuery) ([]ManagedContributor, error) {
	if ctx == nil || query.ActorID.IsZero() || query.TenantID.IsZero() || query.ContextGeneration < 1 {
		return nil, safe("membership_directory_invalid", nil)
	}
	if d == nil || d.reader == nil {
		return nil, safe("membership_directory_unavailable", nil)
	}
	rows, err := d.reader.ListManagedContributors(ctx, query)
	if errors.Is(err, ErrMembershipManagementNotAuthorized) {
		return nil, safe("membership_directory_not_authorized", nil)
	}
	if err != nil {
		return nil, safe("membership_directory_unavailable", err)
	}
	result := make([]ManagedContributor, len(rows))
	for index, row := range rows {
		if row.MembershipID.IsZero() || row.Email == "" || row.ContextGeneration < 1 {
			return nil, safe("membership_directory_unavailable", nil)
		}
		result[index] = row
	}
	return result, nil
}
