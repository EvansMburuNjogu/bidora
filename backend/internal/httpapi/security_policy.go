package httpapi

import (
	"bidora/backend/internal/identity/securitypolicy"
	"context"
	"errors"
)

type SecurityPolicyResolver func(context.Context) (securitypolicy.Policy, error)

func (m *BrowserSessionMiddleware) ConfigureSecurityPolicy(read SecurityPolicyResolver) error {
	if m == nil || read == nil {
		return errors.New("security_policy_resolver_invalid")
	}
	m.securityPolicy = read
	return nil
}
