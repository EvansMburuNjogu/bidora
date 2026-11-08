package platformusers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"bidora/backend/internal/identity/email"
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	invite "bidora/backend/internal/platformadmin/invitation"
)

var at = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
var actor = app.ID{1}
var target = app.ID{3}

type clock struct{}

func (clock) Now() time.Time { return at }

type ids struct{ n byte }

func (g *ids) NewID() (app.ID, error) { g.n++; return app.ID{100, g.n}, nil }

type authority struct {
	role domain.Role
	age  time.Duration
}

func (a authority) identity(q app.AuthorityQuery) (app.IdentityAuthorityFacts, error) {
	return app.IdentityAuthorityFacts{UserID: q.UserID, AuthEpoch: 1, AffiliationKey: "owner@example.test", AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at.Add(-a.age), IdleExpiresAt: at.Add(time.Hour), AbsoluteExpiresAt: at.Add(time.Hour)}, nil
}
func (a authority) platform(id app.ID) (app.PlatformAuthorityFacts, error) {
	r := a.role
	if r == "" {
		r = domain.PlatformOwner
	}
	return app.PlatformAuthorityFacts{UserID: id, ContextGeneration: 1, AdministratorVersion: 1, RoleKeys: []string{string(r)}}, nil
}

type roleTx struct {
	app.RoleChangeTx
	auth                          authority
	owners                        int64
	targetOwner, disabled         bool
	archived                      bool
	writes, evidence, generations int
	command                       *app.CommandRecord
	stored                        app.IdentityStateChangeResult
}

func (s *roleTx) GetIdentityAuthority(_ context.Context, q app.AuthorityQuery) (app.IdentityAuthorityFacts, error) {
	return s.auth.identity(q)
}
func (s *roleTx) GetPlatformAuthority(_ context.Context, id app.ID) (app.PlatformAuthorityFacts, error) {
	return s.auth.platform(id)
}
func (s *roleTx) GetCommand(context.Context, app.ID, string, [32]byte) (app.CommandRecord, bool, error) {
	if s.command != nil {
		return *s.command, true, nil
	}
	return app.CommandRecord{}, false, nil
}
func (s *roleTx) GetIdentityStateChange(context.Context, app.ID) (app.IdentityStateChangeResult, error) {
	return s.stored, nil
}
func (s *roleTx) LockAdministratorProfile(_ context.Context, id app.ID) (app.AdministratorProfile, error) {
	return app.AdministratorProfile{Removed: s.archived, UserID: id, Version: 1, ContextGeneration: 1}, nil
}
func (s *roleTx) LockAdministratorIdentity(_ context.Context, id app.ID) (app.AdministratorIdentity, error) {
	return app.AdministratorIdentity{UserID: id, AuthEpoch: 1, Disabled: s.disabled}, nil
}
func (s *roleTx) ActiveRoleExists(context.Context, app.ID, domain.Role) (bool, error) {
	return false, nil
}
func (s *roleTx) LockRoleAssignment(_ context.Context, id, user app.ID) (app.RoleAssignment, error) {
	return app.RoleAssignment{ID: id, UserID: user, Role: domain.PlatformOwner, Version: 1}, nil
}
func (s *roleTx) LockGovernance(context.Context) (app.GovernanceState, error) {
	return app.GovernanceState{Version: 1}, nil
}
func (s *roleTx) LockActiveOwnerIDs(context.Context) ([]app.ID, error) {
	if s.targetOwner {
		return []app.ID{actor, target}, nil
	}
	return []app.ID{actor}, nil
}
func (s *roleTx) CountEffectiveOwnerIdentities(_ context.Context, ids []app.ID) (int64, error) {
	if len(ids) == 1 && ids[0] == target {
		if s.targetOwner && !s.disabled {
			return 1, nil
		}
		return 0, nil
	}
	return s.owners, nil
}
func (s *roleTx) InsertRoleAssignment(context.Context, app.ID, app.ID, domain.Role, app.ID, string, time.Time) error {
	s.writes++
	return nil
}
func (s *roleTx) RevokeRoleAssignment(context.Context, app.RoleAssignment, app.ID, string, time.Time) error {
	s.writes++
	return nil
}
func (s *roleTx) RemoveAdministratorProfile(context.Context, app.AdministratorProfile, time.Time) error {
	s.archived = true
	return nil
}
func (s *roleTx) AdvanceAdministratorProfile(context.Context, app.AdministratorProfile, time.Time) error {
	return nil
}
func (s *roleTx) AdvanceIdentityEpoch(context.Context, app.AdministratorIdentity, time.Time) error {
	return nil
}
func (s *roleTx) RevokeSessions(context.Context, app.ID, time.Time) (int64, error) { return 2, nil }
func (s *roleTx) AdvanceOwnerGeneration(context.Context, int64, time.Time) error {
	s.generations++
	return nil
}
func (s *roleTx) InsertRoleChange(_ context.Context, _ app.ID, in app.RoleChangeInput, _ *app.ID, state string, _ time.Time, expires *time.Time) error {
	if state != "EXECUTED" || expires != nil {
		panic("new role changes must never enter approval")
	}
	s.stored = app.IdentityStateChangeResult{TargetUserID: in.TargetUserID, State: state}
	return nil
}
func (s *roleTx) ClaimCommand(_ context.Context, _, _ app.ID, _ string, _ [32]byte, hash [32]byte, id app.ID, status int32, _ time.Time) error {
	s.command = &app.CommandRecord{RequestHash: hash, TargetID: id, ResponseStatus: status}
	return nil
}
func (s *roleTx) AppendRoleChangeEvidence(context.Context, app.RoleChangeEvidence) error {
	s.evidence++
	return nil
}
func (s *roleTx) SetAdministratorDisabled(context.Context, app.AdministratorIdentity, time.Time) error {
	s.writes++
	return nil
}
func (s *roleTx) SetAdministratorActive(context.Context, app.AdministratorIdentity, time.Time) error {
	s.writes++
	return nil
}
func (s *roleTx) InsertIdentityStateChange(_ context.Context, _ app.ID, in app.IdentityStateChangeInput, state string, _ time.Time, expires *time.Time) error {
	if state != "EXECUTED" || expires != nil {
		panic("new account changes must never enter approval")
	}
	s.stored = app.IdentityStateChangeResult{TargetUserID: in.TargetUserID, Action: in.Action, State: state}
	return nil
}
func (s *roleTx) AppendIdentityStateEvidence(context.Context, app.IdentityStateEvidence) error {
	s.evidence++
	return nil
}

type roles struct{ tx *roleTx }

func (s roles) WithinRoleChange(ctx context.Context, fn func(context.Context, app.RoleChangeTx) error) error {
	return fn(ctx, s.tx)
}
func roleInput() app.RoleChangeInput {
	return app.RoleChangeInput{ActorID: actor, SessionID: app.ID{2}, TargetUserID: target, CorrelationID: app.ID{5}, IdempotencyKey: app.ID{6}, PlatformContextGeneration: 1, ExpectedAdministratorVersion: 1, ExpectedTargetContextGeneration: 1, Action: app.AssignRole, Role: domain.PlatformOwner, Reason: "Delegate platform ownership"}
}
func TestOwnerRoleAssignmentExecutesDirectlyAndReplaysWithoutAnotherMutation(t *testing.T) {
	tx := &roleTx{owners: 1}
	s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
	in := roleInput()
	r, e := s.Change(context.Background(), in)
	if e != nil || r.State != "EXECUTED" || r.RevokedSessions != 2 || tx.writes != 1 || tx.evidence != 1 || tx.generations != 1 {
		t.Fatal("direct role change failed", r, e)
	}
	r, e = s.Change(context.Background(), in)
	if e != nil || !r.Replay || r.State != "EXECUTED" || tx.writes != 1 || tx.evidence != 1 {
		t.Fatal("role replay mutated state", r, e)
	}
}
func TestLastEffectiveOwnerCannotBeRevoked(t *testing.T) {
	tx := &roleTx{owners: 1, targetOwner: true}
	s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
	in := roleInput()
	in.Action = app.RevokeRole
	in.AssignmentID = app.ID{4}
	in.ExpectedAssignmentVersion = 1
	_, e := s.Change(context.Background(), in)
	if !errors.Is(e, app.ErrInvitationConflict) || tx.writes != 0 {
		t.Fatal("last owner was not protected", e)
	}
}
func TestDirectOwnerRoleChangeRequiresFreshOwnerAuthority(t *testing.T) {
	for _, auth := range []authority{{role: domain.SupportAdministrator}, {age: 16 * time.Minute}} {
		tx := &roleTx{owners: 2, auth: auth}
		s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
		_, e := s.Change(context.Background(), roleInput())
		if e == nil || tx.writes != 0 {
			t.Fatal("unauthorized role mutation")
		}
	}
}
func identityInput() app.IdentityStateChangeInput {
	return app.IdentityStateChangeInput{ActorID: actor, SessionID: app.ID{2}, TargetUserID: target, CorrelationID: app.ID{5}, IdempotencyKey: app.ID{6}, PlatformContextGeneration: 1, ExpectedAdministratorVersion: 1, ExpectedTargetContextGeneration: 1, Action: app.DisableAdministrator, Reason: "Remove platform access"}
}
func TestOwnerDisableAndReactivateAreDirectAndProtectContinuity(t *testing.T) {
	for _, reactivate := range []bool{false, true} {
		tx := &roleTx{owners: 2, targetOwner: true, disabled: reactivate}
		s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
		in := identityInput()
		if reactivate {
			in.Action = app.ReactivateAdministrator
			tx.owners = 1
		}
		r, e := s.ChangeIdentityState(context.Background(), in)
		if e != nil || r.State != "EXECUTED" || tx.writes != 1 || tx.evidence != 1 || tx.generations != 1 {
			t.Fatal("direct access change failed", r, e)
		}
	}
	tx := &roleTx{owners: 1, targetOwner: true}
	s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
	_, e := s.ChangeIdentityState(context.Background(), identityInput())
	if !errors.Is(e, app.ErrInvitationConflict) || tx.writes != 0 {
		t.Fatal("last owner disabled", e)
	}
}
func TestRemoveAdministratorArchivesActiveAndDisabledUsersAndReplays(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		tx := &roleTx{owners: 1, disabled: disabled}
		s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
		in := identityInput()
		in.Action = app.RemoveAdministrator
		r, e := s.ChangeIdentityState(context.Background(), in)
		if e != nil || r.State != "EXECUTED" || !tx.archived || tx.evidence != 1 || r.RevokedSessions != 2 {
			t.Fatal("removal did not archive, revoke and audit", e)
		}
		r, e = s.ChangeIdentityState(context.Background(), in)
		if e != nil || !r.Replay || tx.evidence != 1 {
			t.Fatal("removal replay mutated evidence", e)
		}
	}
	for _, tx := range []*roleTx{{owners: 1, targetOwner: true}, {owners: 2, auth: authority{role: domain.SecurityAuditAdministrator}}} {
		s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
		in := identityInput()
		in.Action = app.RemoveAdministrator
		if _, e := s.ChangeIdentityState(context.Background(), in); e == nil || tx.archived {
			t.Fatal("last-owner or permission safeguard bypassed")
		}
	}
}
func TestSecurityAuditorCannotDirectlyDisableOwnerAndSelfDisableIsDenied(t *testing.T) {
	tx := &roleTx{owners: 2, targetOwner: true, auth: authority{role: domain.SecurityAuditAdministrator}}
	s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
	_, e := s.ChangeIdentityState(context.Background(), identityInput())
	if !errors.Is(e, app.ErrPlatformAccessForbidden) || tx.writes != 0 {
		t.Fatal("non-owner disabled owner", e)
	}
	tx.auth = authority{}
	in := identityInput()
	in.TargetUserID = actor
	_, e = s.ChangeIdentityState(context.Background(), in)
	if !errors.Is(e, app.ErrPlatformAccessForbidden) || tx.writes != 0 {
		t.Fatal("self disable was permitted", e)
	}
}

type invitationTx struct {
	app.InvitationTx
	auth     authority
	owners   int64
	record   app.InvitationRecord
	evidence int
}

func (s *invitationTx) GetIdentityAuthority(_ context.Context, q app.AuthorityQuery) (app.IdentityAuthorityFacts, error) {
	return s.auth.identity(q)
}
func (s *invitationTx) GetPlatformAuthority(_ context.Context, id app.ID) (app.PlatformAuthorityFacts, error) {
	return s.auth.platform(id)
}
func (s *invitationTx) GetCommand(context.Context, app.ID, string, [32]byte) (app.CommandRecord, bool, error) {
	return app.CommandRecord{}, false, nil
}
func (s *invitationTx) LockGovernance(context.Context) (app.GovernanceState, error) {
	return app.GovernanceState{Version: 1, FirstAdditionalOwnerExceptionConsumed: true}, nil
}
func (s *invitationTx) LockActiveOwnerIDs(context.Context) ([]app.ID, error) {
	return []app.ID{actor}, nil
}
func (s *invitationTx) CountEffectiveOwnerIdentities(context.Context, []app.ID) (int64, error) {
	return s.owners, nil
}
func (s *invitationTx) ExpireReplacedInvitationChanges(context.Context, string, time.Time) error {
	return nil
}
func (s *invitationTx) ReplaceOpenInvitations(context.Context, string, time.Time) error { return nil }
func (s *invitationTx) InsertInvitation(_ context.Context, r app.InvitationRecord, hash *[32]byte) error {
	if hash == nil {
		panic("direct invitation requires token hash")
	}
	s.record = r
	return nil
}
func (s *invitationTx) ClaimCommand(context.Context, app.ID, app.ID, string, [32]byte, [32]byte, app.ID, int32, time.Time) error {
	return nil
}
func (s *invitationTx) AppendInvitationEvidence(context.Context, app.InvitationEvidence) error {
	s.evidence++
	return nil
}

type invitations struct{ tx *invitationTx }

func (s invitations) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	return fn(ctx, s.tx)
}

type delivery struct{ calls int }

func (d *delivery) DeliverPlatformInvitation(context.Context, email.Address, invite.Token, domain.Role, time.Time) error {
	d.calls++
	return nil
}
func TestEveryRoleInvitationIsActiveDirectAndEvidencedForOneOrMultipleOwners(t *testing.T) {
	for _, owners := range []int64{1, 2} {
		for _, role := range domainRoles() {
			tx := &invitationTx{owners: owners}
			d := &delivery{}
			s, _ := app.NewInvitationService(invitations{tx}, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, d)
			r, e := s.Issue(context.Background(), app.IssueInvitationInput{ActorID: actor, SessionID: app.ID{2}, CorrelationID: app.ID{5}, IdempotencyKey: app.ID{6}, PlatformContextGeneration: 1, Email: "new@example.test", Role: role, Reason: "Invite administrator"})
			if e != nil || r.State != app.InvitationActive || tx.record.AuthorizationPath != app.InvitationOwnerDirect || d.calls != 1 || tx.evidence != 1 || tx.record.ExpiresAt.Sub(at) != 72*time.Hour {
				t.Fatal("invitation required review or lacked delivery/evidence", role, e)
			}
		}
	}
}
func domainRoles() []domain.Role {
	return []domain.Role{domain.PlatformOwner, domain.OperationsAdministrator, domain.BillingAdministrator, domain.IntegrationAdministrator, domain.SupportAdministrator, domain.SecurityAuditAdministrator}
}
func TestDirectInvitationStillRejectsStaleOrNonOwnerAuthority(t *testing.T) {
	for _, auth := range []authority{{role: domain.SupportAdministrator}, {age: 16 * time.Minute}} {
		tx := &invitationTx{owners: 2, auth: auth}
		d := &delivery{}
		s, _ := app.NewInvitationService(invitations{tx}, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, d)
		_, e := s.Issue(context.Background(), app.IssueInvitationInput{ActorID: actor, SessionID: app.ID{2}, CorrelationID: app.ID{5}, IdempotencyKey: app.ID{6}, PlatformContextGeneration: 1, Email: "new@example.test", Role: domain.PlatformOwner, Reason: "Invite administrator"})
		if e == nil || d.calls != 0 || tx.evidence != 0 {
			t.Fatal("unauthorized invitation created")
		}
	}
}
