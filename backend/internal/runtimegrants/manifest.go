// Package runtimegrants applies reviewed, query-derived database privileges to a
// dedicated application runtime role. It is deployment tooling and is not used
// by API startup.
package runtimegrants

import (
	"errors"
	"sort"
	"strings"
)

const Version = 1

type Privilege string

const (
	Select Privilege = "SELECT"
	Insert Privilege = "INSERT"
	Update Privilege = "UPDATE"
	Delete Privilege = "DELETE"
)

type TableGrant struct {
	Table      string
	Privileges []Privilege
}

type Manifest struct {
	Plane                string
	CompatibilitySet     string
	CompatibilityVersion int64
	Schemas              []string
	Tables               []TableGrant
}

var control = map[string]string{
	"bidora_control_audit.audit_events":                         "I",
	"bidora_control_eventing.ai_acknowledgement_dispatch_holds": "SI",
	"bidora_control_eventing.inbox_events":                      "I",
	"bidora_control_eventing.outbox_events":                     "SIU",
	"bidora_control_meta.schema_compatibility":                  "S",
	"bidora_tenancy.database_clusters":                          "S",
	"bidora_tenancy.tenant_database_assignments":                "S",
	"bidora_tenancy.tenant_schema_versions":                     "S",
	"bidora_tenancy.tenants":                                    "SIU",
	"bidora_tenancy.platform_tenant_commands":                   "SI",
	"bidora_support.cases":                                      "SIU",
	"bidora_support.case_messages":                              "SI",
	"bidora_support.case_events":                                "I",
	"bidora_support.case_commands":                              "SI",
	"commercial.additional_licence_grants":                      "SI",
	"commercial.ai_reservation_request_outcomes":                "SI",
	"commercial.ai_usage_reservation_transitions":               "SI",
	"commercial.ai_usage_reservations":                          "SIU",
	"commercial.credit_notes":                                   "SI",
	"commercial.financial_change_decisions":                     "I",
	"commercial.financial_changes":                              "SIU",
	"commercial.financial_commands":                             "SI",
	"commercial.financial_number_heads":                         "SIU",
	"commercial.invoice_entitlement_snapshots":                  "SI",
	"commercial.invoice_lines":                                  "SI",
	"commercial.invoices":                                       "SIU",
	"commercial.payment_observations":                           "SI",
	"commercial.payment_operations":                             "SIU",
	"commercial.payment_reconciliation_claims":                  "SI",
	"commercial.payment_reconciliation_outcomes":                "SI",
	"commercial.payment_reconciliation_requests":                "SI",
	"commercial.payment_reversal_effects":                       "SI",
	"commercial.payment_settlement_effects":                     "SI",
	"commercial.receipts":                                       "SI",
	"commercial.refund_operations":                              "SI",
	"commercial.subscription_access_holds":                      "SI",
	"commercial.subscription_commands":                          "SI",
	"commercial.subscription_terms":                             "SI",
	"commercial.subscription_transitions":                       "I",
	"commercial.subscriptions":                                  "SIU",
	"commercial.tax_profiles":                                   "S",
	"commercial.trial_lifetime_markers":                         "SI",
	"commercial.usage_periods":                                  "SIU",
	"identity.account_type_selection_commands":                  "SI",
	"identity.authentication_identities":                        "SIU",
	"identity.challenges":                                       "SIU",
	"identity.external_identities":                              "SIU",
	"identity.mfa_factors":                                      "SIU",
	"identity.mfa_recovery_codes":                               "SIU",
	"identity.mfa_transactions":                                 "SIU",
	"identity.password_recovery_challenges":                     "SIU",
	"identity.session_revocation_commands":                      "SI",
	"identity.sessions":                                         "SIU",
	"identity.users":                                            "SIU",
	"platform_administration.administrator_changes":             "SIU",
	"platform_administration.administrator_invitations":         "SIU",
	"platform_administration.administrator_profiles":            "SIU",
	"platform_administration.commands":                          "SI",
	"platform_administration.governance_state":                  "SU",
	"platform_administration.role_assignments":                  "SIU",
	"platform_settings.session_policy":                          "SU",
	"platform_settings.session_policy_commands":                 "SI",
	"platform_settings.platform_information":                    "SU",
	"platform_settings.platform_information_commands":           "SI",
	"platform_settings.integration_activation_history":          "I",
	"platform_settings.integration_changes":                     "SIU",
	"platform_settings.integration_commands":                    "SIU",
	"platform_settings.google_versions":                         "SI",
	"platform_settings.google_head":                             "SU",
	"platform_settings.google_tests":                            "SI",
	"platform_settings.google_commands":                         "SIU",
	"notification.templates":                                    "SU",
	"notification.template_versions":                            "SI",
	"notification.template_commands":                            "SIU",
	"platform_settings.integration_heads":                       "SU",
	"platform_settings.integration_test_attempts":               "SI",
	"platform_settings.integration_versions":                    "SIU",
	"platform_settings.openai_versions":                         "SI",
	"platform_settings.smtp_versions":                           "SI",
	"platform_settings.countries":                               "SIU",
	"platform_settings.country_versions":                        "S",
	"platform_settings.operational_policy":                      "SU",
	"platform_settings.operational_policy_versions":             "SI",
	"platform_settings.regional_policy_commands":                "SI",
	"commercial.payment_callback_hints":                         "SI",
	"commercial.platform_plan_versions":                         "SI",
	"commercial.platform_plan_heads":                            "SU",
	"commercial.platform_plan_commands":                         "SI",
	"commercial.platform_subscription_commands":                 "SI",
	"platform_settings.pesapal_versions":                        "SI",
	"bidora_control_audit.request_metadata":                     "I",
	"bidora_control_audit.platform_request_metadata_v1":         "S",
	"bidora_control_audit.platform_action_details_v1":           "S",
	"platform_settings.smtp_secrets":                            "SI",
	"notification.delivery_attempt_results":                     "SI",
	"notification.delivery_attempts":                            "SI",
	"notification.message_snapshots":                            "SID",
	"notification.notifications":                                "SI",
	"tenancy.invitations":                                       "SIU",
	"tenancy.organization_creation_commands":                    "SI",
	"tenancy.organization_creation_lifetime_markers":            "S",
	"tenancy.organization_creation_operations":                  "SI",
	"tenancy.provisioning_jobs":                                 "I",
	"tenancy.membership_offboarding_events":                     "SI",
	"tenancy.membership_offboarding_operations":                 "SIU",
	"tenancy.memberships":                                       "SIU",
	"tenancy.role_grants":                                       "SIU",
}

var tenant = map[string]string{
	"ai.document_measurement_attempt_outcomes":                       "SI",
	"ai.document_measurement_attempts":                               "SI",
	"ai.document_measurement_entitlements":                           "SI",
	"ai.document_measurements":                                       "SI",
	"ai.extraction_runs":                                             "SIU",
	"ai.proposal_draft_citations":                                    "SI",
	"ai.proposal_draft_inputs":                                       "SI",
	"ai.proposal_draft_runs":                                         "SIU",
	"ai.proposal_draft_suggestions":                                  "SIU",
	"ai.requirement_suggestions":                                     "SIU",
	"ai.response_draft_citations":                                    "SI",
	"ai.response_draft_inputs":                                       "SI",
	"ai.response_draft_runs":                                         "SIU",
	"ai.response_draft_suggestions":                                  "SI",
	"ai.response_gap_suggestions":                                    "SI",
	"bid.archive_events":                                             "SI",
	"bid.assignment_guards":                                          "SIU",
	"bid.decisions":                                                  "SI",
	"bid.external_action_evidence":                                   "SI",
	"bid.internal_review_decisions":                                  "SI",
	"bid.membership_ownership_transfer_events":                       "SI",
	"bid.proposal_approvals":                                         "SI",
	"bid.proposal_package_manifests":                                 "SI",
	"bid.proposal_response_versions":                                 "SI",
	"bid.proposal_versions":                                          "SI",
	"bid.proposals":                                                  "SIU",
	"bid.reported_outcomes":                                          "SI",
	"bid.requirement_evidence":                                       "SI",
	"bid.requirement_versions":                                       "SI",
	"bid.requirements":                                               "SIU",
	"bid.response_evidence":                                          "SI",
	"bid.response_sections":                                          "SIU",
	"bid.response_versions":                                          "SI",
	"bid.review_assignments":                                         "SI",
	"bid.review_comments":                                            "SI",
	"bid.review_threads":                                             "SIU",
	"bid.submission_confirmations":                                   "SI",
	"bid.workspaces":                                                 "SIU",
	"bidora_audit.audit_events":                                      "I",
	"bidora_commands.idempotency_records":                            "SIU",
	"bidora_entitlement.ai_usage_checkpoint_transitions":             "I",
	"bidora_entitlement.ai_usage_checkpoints":                        "SIU",
	"bidora_entitlement.ai_usage_completion_acknowledgements":        "SI",
	"bidora_entitlement.ai_usage_completion_reconciliation_attempts": "SI",
	"bidora_entitlement.ai_usage_completion_reconciliation_outcomes": "SI",
	"bidora_entitlement.ai_usage_completion_reconciliations":         "SIU",
	"bidora_entitlement.ai_usage_completion_requests":                "SI",
	"bidora_eventing.inbox_events":                                   "SI",
	"bidora_eventing.outbox_events":                                  "SIU",
	"bidora_meta.schema_compatibility":                               "S",
	"bidora_opportunity.opportunities":                               "SIU",
	"bidora_opportunity.source_versions":                             "SI",
	"bidora_tenant.organization_profile":                             "S",
	"documents.document_versions":                                    "SIU",
	"documents.documents":                                            "SIU",
	"documents.upload_sessions":                                      "SIU",
	"jobs.jobs":                                                      "SIU",
	"organization.company_profiles":                                  "SIU",
	"organization.letterhead_activation_events":                      "I",
	"organization.letterhead_configuration":                          "SU",
	"organization.letterhead_versions":                               "SI",
}

func Load(plane string) (Manifest, error) {
	var source map[string]string
	var result Manifest
	switch plane {
	case "control":
		source = control
		result = Manifest{Plane: plane, CompatibilitySet: "control_registry", CompatibilityVersion: 57}
	case "tenant":
		source = tenant
		result = Manifest{Plane: plane, CompatibilitySet: "tenant_common+tenant_tenderpreneur", CompatibilityVersion: 9_000_034}
	default:
		return Manifest{}, errors.New("runtime grant plane is not allowlisted")
	}
	schemaSet := map[string]struct{}{}
	for table, compact := range source {
		parts := strings.Split(table, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return Manifest{}, errors.New("runtime grant table is invalid")
		}
		schemaSet[parts[0]] = struct{}{}
		grant := TableGrant{Table: table}
		for _, item := range []struct {
			code  byte
			value Privilege
		}{{'S', Select}, {'I', Insert}, {'U', Update}, {'D', Delete}} {
			if strings.ContainsRune(compact, rune(item.code)) {
				grant.Privileges = append(grant.Privileges, item.value)
			}
		}
		if len(grant.Privileges) == 0 {
			return Manifest{}, errors.New("runtime grant privilege is invalid")
		}
		result.Tables = append(result.Tables, grant)
	}
	for schema := range schemaSet {
		result.Schemas = append(result.Schemas, schema)
	}
	sort.Strings(result.Schemas)
	sort.Slice(result.Tables, func(i, j int) bool { return result.Tables[i].Table < result.Tables[j].Table })
	return result, nil
}
