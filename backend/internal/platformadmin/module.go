// Package platformadmin exposes the inbound application facade for the global
// Platform Administration bounded context. Transport and composition code need
// not depend on persistence or generated repositories.
package platformadmin

import (
	"context"
	"errors"

	"bidora/backend/internal/platformadmin/application"
)

type Module struct {
	bootstrap   *application.BootstrapService
	authority   *application.AuthorityService
	invitations *application.InvitationService
	roles       *application.RoleChangeService
	reads       *application.GovernanceReadService
}

func NewModule(bootstrap application.BootstrapUnitOfWork, authority application.AuthorityUnitOfWork) (*Module, error) {
	bootstrapService, err := application.NewBootstrapService(bootstrap)
	if err != nil {
		return nil, err
	}
	authorityService, err := application.NewAuthorityService(authority)
	if err != nil {
		return nil, err
	}
	return &Module{bootstrap: bootstrapService, authority: authorityService}, nil
}

// NewModuleWithReads composes the read-only Platform Administration catalogue,
// administrator directory and immutable assignment-history application facade.
func NewModuleWithReads(bootstrap application.BootstrapUnitOfWork, authority application.AuthorityUnitOfWork, reads application.GovernanceReadUnitOfWork) (*Module, error) {
	module, err := NewModule(bootstrap, authority)
	if err != nil {
		return nil, err
	}
	readService, err := application.NewGovernanceReadService(reads)
	if err != nil {
		return nil, err
	}
	module.reads = readService
	return module, nil
}

// NewModuleWithInvitations composes the application facade without selecting an
// HTTP or SMTP adapter. The caller supplies explicit delivery and time/ID ports.
func NewModuleWithInvitations(bootstrap application.BootstrapUnitOfWork, authority application.AuthorityUnitOfWork, invitations application.InvitationUnitOfWork, reads application.GovernanceReadUnitOfWork, tokens application.InvitationTokenGenerator, ids application.InvitationIDGenerator, clock application.InvitationClock, delivery application.PlatformInvitationDelivery) (*Module, error) {
	module, err := NewModuleWithReads(bootstrap, authority, reads)
	if err != nil {
		return nil, err
	}
	service, err := application.NewInvitationService(invitations, tokens, ids, clock, delivery)
	if err != nil {
		return nil, err
	}
	module.invitations = service
	return module, nil
}

// NewModuleWithAdministratorChanges adds fixed-role and identity-state change
// services without selecting which bounded HTTP routes are mounted.
func NewModuleWithAdministratorChanges(bootstrap application.BootstrapUnitOfWork, authority application.AuthorityUnitOfWork, roles application.RoleChangeUnitOfWork, reads application.GovernanceReadUnitOfWork, ids application.InvitationIDGenerator, clock application.InvitationClock) (*Module, error) {
	module, err := NewModuleWithReads(bootstrap, authority, reads)
	if err != nil {
		return nil, err
	}
	service, err := application.NewRoleChangeService(roles, ids, clock)
	if err != nil {
		return nil, err
	}
	module.roles = service
	return module, nil
}

func NewModuleWithAdministration(bootstrap application.BootstrapUnitOfWork, authority application.AuthorityUnitOfWork, invitations application.InvitationUnitOfWork, roles application.RoleChangeUnitOfWork, reads application.GovernanceReadUnitOfWork, tokens application.InvitationTokenGenerator, ids application.InvitationIDGenerator, clock application.InvitationClock, delivery application.PlatformInvitationDelivery) (*Module, error) {
	module, err := NewModuleWithInvitations(bootstrap, authority, invitations, reads, tokens, ids, clock, delivery)
	if err != nil {
		return nil, err
	}
	service, err := application.NewRoleChangeService(roles, ids, clock)
	if err != nil {
		return nil, err
	}
	if securityDelivery, ok := delivery.(application.PlatformSecurityDelivery); ok {
		if err := service.EnableSecurityDelivery(securityDelivery); err != nil {
			return nil, err
		}
	}
	module.roles = service
	return module, nil
}

func (m *Module) Roles(ctx context.Context, auth application.ReadAuthority) ([]application.RoleCatalogueView, error) {
	if m == nil || m.reads == nil {
		return nil, errors.New("platform_module_unavailable")
	}
	return m.reads.Roles(ctx, auth)
}
func (m *Module) Administrators(ctx context.Context, auth application.ReadAuthority, filter application.AdministratorFilter, page application.PageCursor) (application.AdministratorList, error) {
	if m == nil || m.reads == nil {
		return application.AdministratorList{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Administrators(ctx, auth, filter, page)
}
func (m *Module) Administrator(ctx context.Context, auth application.ReadAuthority, userID application.ID) (application.AdministratorView, error) {
	if m == nil || m.reads == nil {
		return application.AdministratorView{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Administrator(ctx, auth, userID)
}
func (m *Module) Invitation(ctx context.Context, auth application.ReadAuthority, id application.ID) (application.InvitationView, error) {
	if m == nil || m.reads == nil {
		return application.InvitationView{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Invitation(ctx, auth, id)
}
func (m *Module) Invitations(ctx context.Context, auth application.ReadAuthority, page application.PageCursor) (application.InvitationList, error) {
	if m == nil || m.reads == nil {
		return application.InvitationList{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Invitations(ctx, auth, page)
}
func (m *Module) AssignmentHistory(ctx context.Context, auth application.ReadAuthority, userID application.ID, page application.PageCursor) (application.AssignmentHistory, error) {
	if m == nil || m.reads == nil {
		return application.AssignmentHistory{}, errors.New("platform_module_unavailable")
	}
	return m.reads.AssignmentHistory(ctx, auth, userID, page)
}
func (m *Module) Changes(ctx context.Context, auth application.ReadAuthority, state *string, page application.PageCursor) (application.ChangeList, error) {
	if m == nil || m.reads == nil {
		return application.ChangeList{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Changes(ctx, auth, state, page)
}
func (m *Module) Change(ctx context.Context, auth application.ReadAuthority, changeID application.ID) (application.ChangeView, error) {
	if m == nil || m.reads == nil {
		return application.ChangeView{}, errors.New("platform_module_unavailable")
	}
	return m.reads.Change(ctx, auth, changeID)
}

func (m *Module) Bootstrap(ctx context.Context, commit application.BootstrapCommit) (application.BootstrapResult, error) {
	if m == nil || m.bootstrap == nil {
		return application.BootstrapResult{}, errors.New("platform_module_unavailable")
	}
	return m.bootstrap.Bootstrap(ctx, commit)
}

func (m *Module) GetAuthority(ctx context.Context, query application.AuthorityQuery) (application.AuthorityFacts, error) {
	if m == nil || m.authority == nil {
		return application.AuthorityFacts{}, errors.New("platform_module_unavailable")
	}
	return m.authority.GetAuthority(ctx, query)
}

func (m *Module) ApproveOwnerRoleChange(ctx context.Context, input application.OwnerRoleDispositionInput) (application.OwnerRoleDispositionResult, error) {
	if m == nil || m.roles == nil {
		return application.OwnerRoleDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.ApproveOwnerRoleChange(ctx, input)
}
func (m *Module) RejectOwnerRoleChange(ctx context.Context, input application.OwnerRoleDispositionInput) (application.OwnerRoleDispositionResult, error) {
	if m == nil || m.roles == nil {
		return application.OwnerRoleDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.RejectOwnerRoleChange(ctx, input)
}
func (m *Module) ApproveOwnerIdentityStateChange(ctx context.Context, input application.OwnerIdentityDispositionInput) (application.OwnerIdentityDispositionResult, error) {
	if m == nil || m.roles == nil {
		return application.OwnerIdentityDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.ApproveOwnerIdentityStateChange(ctx, input)
}
func (m *Module) RejectOwnerIdentityStateChange(ctx context.Context, input application.OwnerIdentityDispositionInput) (application.OwnerIdentityDispositionResult, error) {
	if m == nil || m.roles == nil {
		return application.OwnerIdentityDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.RejectOwnerIdentityStateChange(ctx, input)
}
func (m *Module) RevokeAdministratorSessions(ctx context.Context, input application.AdministratorSessionRevocationInput) (application.AdministratorSessionRevocationResult, error) {
	if m == nil || m.roles == nil {
		return application.AdministratorSessionRevocationResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.RevokeAdministratorSessions(ctx, input)
}
func (m *Module) ChangeIdentityState(ctx context.Context, input application.IdentityStateChangeInput) (application.IdentityStateChangeResult, error) {
	if m == nil || m.roles == nil {
		return application.IdentityStateChangeResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.ChangeIdentityState(ctx, input)
}
func (m *Module) ChangeRole(ctx context.Context, input application.RoleChangeInput) (application.RoleChangeResult, error) {
	if m == nil || m.roles == nil {
		return application.RoleChangeResult{}, errors.New("platform_module_unavailable")
	}
	return m.roles.Change(ctx, input)
}

func (m *Module) IssueInvitation(ctx context.Context, input application.IssueInvitationInput) (application.IssueInvitationResult, error) {
	if m == nil || m.invitations == nil {
		return application.IssueInvitationResult{}, errors.New("platform_module_unavailable")
	}
	return m.invitations.Issue(ctx, input)
}
func (m *Module) CancelInvitation(ctx context.Context, input application.CancelInvitationInput) error {
	if m == nil || m.invitations == nil {
		return errors.New("platform_module_unavailable")
	}
	return m.invitations.Cancel(ctx, input)
}
func (m *Module) ApproveOwnerInvitation(ctx context.Context, input application.OwnerInvitationDispositionInput) (application.OwnerInvitationDispositionResult, error) {
	if m == nil || m.invitations == nil {
		return application.OwnerInvitationDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.invitations.ApproveOwnerInvitation(ctx, input)
}
func (m *Module) RejectOwnerInvitation(ctx context.Context, input application.OwnerInvitationDispositionInput) (application.OwnerInvitationDispositionResult, error) {
	if m == nil || m.invitations == nil {
		return application.OwnerInvitationDispositionResult{}, errors.New("platform_module_unavailable")
	}
	return m.invitations.RejectOwnerInvitation(ctx, input)
}
func (m *Module) AcceptInvitation(ctx context.Context, input application.AcceptInvitationInput) (application.AcceptInvitationResult, error) {
	if m == nil || m.invitations == nil {
		return application.AcceptInvitationResult{}, errors.New("platform_module_unavailable")
	}
	return m.invitations.Accept(ctx, input)
}
