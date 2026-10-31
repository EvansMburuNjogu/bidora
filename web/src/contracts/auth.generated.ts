// Generated from backend/api/openapi/bidora.openapi.json.
// Run `npm run contracts:generate`; do not edit by hand.

export interface RegistrationRequest {
  first_name: string;
  last_name: string;
  email: string;
  password: string;
}

export const ACCOUNT_TYPES = ["UNASSIGNED","TENDERPRENEUR","PROCUREMENT","PLATFORM_ADMIN"] as const;
export type AccountType = "UNASSIGNED" | "TENDERPRENEUR" | "PROCUREMENT" | "PLATFORM_ADMIN";

export const SELF_SELECTABLE_ACCOUNT_TYPES = ["TENDERPRENEUR","PROCUREMENT"] as const;
export type SelfSelectableAccountType = "TENDERPRENEUR" | "PROCUREMENT";

export interface AccountTypeSelectionRequest {
  account_type: SelfSelectableAccountType;
}

export const PORTAL_KEYS = ["TENDERPRENEUR","PROCUREMENT","PLATFORM_ADMIN"] as const;
export type PortalKey = "TENDERPRENEUR" | "PROCUREMENT" | "PLATFORM_ADMIN";

export const PORTAL_PATHS = ["/tenderpreneur","/procurement","/platform"] as const;
export type PortalPath = "/tenderpreneur" | "/procurement" | "/platform";

export const ASSURANCE_LEVELS = ["AAL1","AAL2"] as const;
export type AssuranceLevel = "AAL1" | "AAL2";

export const REQUIRED_ACTIONS = ["SELECT_ACCOUNT_TYPE","ESTABLISH_TENDERPRENEUR_MEMBERSHIP","ENROLL_PLATFORM_MFA","COMPLETE_PLATFORM_STEP_UP"] as const;
export type RequiredAction = "SELECT_ACCOUNT_TYPE" | "ESTABLISH_TENDERPRENEUR_MEMBERSHIP" | "ENROLL_PLATFORM_MFA" | "COMPLETE_PLATFORM_STEP_UP";

export interface SessionBootstrapResponse {
  data: { csrf_token: string };
  meta: { trace_id: string };
}

export interface PortalDestination {
  key: PortalKey;
  path: PortalPath;
}

export interface SessionSummary {
  assurance_level: AssuranceLevel;
  idle_expires_at: string;
  absolute_expires_at: string;
}

export interface MePortalResponse {
  data: {
    account_type: AccountType;
    portal: PortalDestination | null;
    platform_context_generation: number | null;
    capabilities: string[];
    session: SessionSummary;
    required_actions: RequiredAction[];
  };
  meta: { trace_id: string };
}

export interface ActiveMembership {
  membership_id: string;
  tenant_id: string;
  tenant_type: "TENDERPRENEUR";
  tenant_name: string;
  context_generation: number;
  capabilities: string[];
}

export interface MembershipDirectoryResponse {
  data: { memberships: ActiveMembership[] };
  meta: Record<string, unknown>;
}

export interface SessionInventoryItem {
  session_id: string;
  rotated_from_session_id: string | null;
  current: boolean;
  created_at: string;
  last_seen_at: string;
  idle_expires_at: string;
  absolute_expires_at: string;
}

export interface SessionInventoryResponse {
  data: {
    active_session_count: number;
    other_session_count: number;
    sessions: SessionInventoryItem[];
    current_session_logout_available: boolean;
    other_session_revocation_available: boolean;
    revoke_all_available: boolean;
    revocation_policy_configured: boolean;
    device_metadata_collected: false;
    blockers: string[];
    observed_at: string;
  };
  meta: { trace_id: string };
}

export interface MFAReadinessResponse {
  data: {
    email_verified: boolean;
    account_disabled: boolean;
    password_credential_present: boolean;
    google_identity_present: boolean;
    mfa_enrollment_available: boolean;
    mfa_enrolled: boolean;
    step_up_available: boolean;
    step_up_policy_configured: boolean;
    session_assurance_provenance_available: boolean;
    recovery_codes_available: boolean;
    session_authenticated_at: string;
    session_age_seconds: number;
    blockers: string[];
    observed_at: string;
  };
  meta: { trace_id: string };
}

export type MFAEnrollRequest = Record<string, never>;

export interface MFAEnrollmentResponse {
  data: {
    factor_id: string;
    secret: string;
    otpauth_uri: string;
    expires_at: string;
  };
  meta: { trace_id: string };
}

export interface MFAConfirmRequest {
  code: string;
}

export interface MFAActivationResponse {
  data: {
    recovery_codes: string[];
    assurance_level: "AAL2";
    assurance_method: "TOTP";
  };
  meta: { trace_id: string };
}

export const MFA_STEP_UP_METHODS = ["TOTP","RECOVERY_CODE"] as const;
export type MFAStepUpMethod = "TOTP" | "RECOVERY_CODE";

export interface MFAStepUpRequest {
  code: string;
  method: MFAStepUpMethod;
}

export interface MFARecoveryCodesResponse {
  data: { recovery_codes: string[] };
  meta: { trace_id: string };
}

export const ORGANIZATION_COMMERCIAL_PATHS = ["TRIAL","PAID"] as const;
export type OrganizationCommercialPath = "TRIAL" | "PAID";

export interface OrganizationCreationRequest {
  name: string;
  country: string;
  timezone: string;
  locale: string;
  business_type: string;
  primary_industry: string;
  preferred_language: string;
  currency: "KES";
  commercial_path: OrganizationCommercialPath;
}

export interface OrganizationCreationResponse {
  data: { operation_id: string; tenant_id: string; state: "REQUESTED" };
  meta: { trace_id: string };
}
