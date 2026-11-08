package platformadmin

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/platformadmin/application"
	"context"
)

func (m *Module) Preview(ctx context.Context, token string) (application.InvitationPreview, error) {
	return m.invitations.Preview(ctx, token)
}
func (m *Module) SetUpNewAccount(ctx context.Context, token string, password []byte, first, last string, correlation application.ID, hasher identityapp.RegistrationPassword) (string, error) {
	return m.invitations.SetUpNewAccount(ctx, token, password, first, last, correlation, hasher)
}
