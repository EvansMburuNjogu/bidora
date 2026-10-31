// Package domain contains Platform Administration policy values. It has no
// dependency on HTTP, PostgreSQL, provider SDKs or tenant modules.
package domain

import "errors"

type Role string

const (
	PlatformOwner              Role = "PLATFORM_OWNER"
	OperationsAdministrator    Role = "OPERATIONS_ADMINISTRATOR"
	BillingAdministrator       Role = "BILLING_ADMINISTRATOR"
	IntegrationAdministrator   Role = "INTEGRATION_ADMINISTRATOR"
	SupportAdministrator       Role = "SUPPORT_ADMINISTRATOR"
	SecurityAuditAdministrator Role = "SECURITY_AUDIT_ADMINISTRATOR"
)

var roleCapabilities = map[Role][]string{
	PlatformOwner: {
		"platform.admin.read", "platform.admin.invite", "platform.admin.role.assign", "platform.admin.role.revoke",
		"platform.admin.identity.disable", "platform.admin.identity.reactivate", "platform.admin.session.revoke",
		"platform.audit.read", "platform.audit.export", "platform.governance.approve",
	},
	OperationsAdministrator: {
		"platform.tenant.read", "platform.tenant.create", "platform.tenant.provision.retry", "platform.tenant.suspend",
		"platform.tenant.reactivate", "platform.migration.read", "platform.migration.manage", "platform.reference.read",
		"platform.reference.manage", "platform.maintenance.read", "platform.maintenance.manage", "platform.announcement.manage",
		"platform.operations.health.read",
	},
	BillingAdministrator: {
		"platform.plan.read", "platform.plan.manage", "platform.plan.publish", "platform.subscription.read",
		"platform.subscription.manage", "platform.invoice.read", "platform.payment.read", "platform.payment.reconcile",
		"platform.billing.adjustment.request", "platform.billing.export",
	},
	IntegrationAdministrator: {
		"platform.integration.read", "platform.integration.smtp.manage", "platform.integration.smtp.test",
		"platform.integration.smtp.activate", "platform.integration.ai.manage", "platform.integration.ai.test",
		"platform.integration.ai.activate", "platform.integration.pesapal.manage", "platform.integration.pesapal.test",
		"platform.integration.pesapal.activate", "platform.integration.pause", "platform.integration.rollback",
		"platform.integration.failure.read",
	},
	SupportAdministrator: {
		"platform.support.case.read", "platform.support.case.manage", "platform.support.access.request",
		"platform.support.session.use", "platform.support.session.revoke",
	},
	SecurityAuditAdministrator: {
		"platform.security.event.read", "platform.audit.read", "platform.audit.export", "platform.admin.read",
		"platform.admin.identity.disable", "platform.admin.session.revoke", "platform.governance.review",
		"platform.support.session.read", "platform.support.session.revoke", "platform.retention.policy.read",
		"platform.retention.policy.draft",
	},
}

func ParseRole(value string) (Role, error) {
	role := Role(value)
	if _, ok := roleCapabilities[role]; !ok {
		return "", errors.New("platform_role_invalid")
	}
	return role, nil
}

func Roles() []Role {
	return []Role{PlatformOwner, OperationsAdministrator, BillingAdministrator, IntegrationAdministrator, SupportAdministrator, SecurityAuditAdministrator}
}

func Capabilities(role Role) ([]string, error) {
	values, ok := roleCapabilities[role]
	if !ok {
		return nil, errors.New("platform_role_invalid")
	}
	if role != PlatformOwner {
		return append([]string(nil), values...), nil
	}
	// The fixed Platform Owner now includes every fixed Platform role capability.
	// This does not create tenant membership, enable an unmounted operation, or
	// bypass the role-specific maker/checker and financial guards in use cases.
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, assignedRole := range Roles() {
		for _, capability := range roleCapabilities[assignedRole] {
			if !seen[capability] {
				seen[capability] = true
				result = append(result, capability)
			}
		}
	}
	return result, nil
}

func Grants(role Role, capability string) bool {
	if capability == "" {
		return false
	}
	values, err := Capabilities(role)
	if err != nil {
		return false
	}
	for _, value := range values {
		if value == capability {
			return true
		}
	}
	return false
}
