import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const sourcePath = resolve(root, "../backend/api/openapi/bidora.openapi.json");
const outputPath = resolve(root, "src/contracts/auth.generated.ts");
const check = process.argv.includes("--check");

const document = JSON.parse(await readFile(sourcePath, "utf8"));

function responseSchema(path, method = "get", status = "200") {
  const response = document.paths?.[path]?.[method]?.responses?.[status]?.content?.["application/json"]?.schema;
  if (!response) throw new Error(`missing canonical ${status} response schema for ${method.toUpperCase()} ${path}`);
  if (!response.$ref) return { name: path, schema: response };
  if (!response.$ref.startsWith("#/components/schemas/")) {
    throw new Error(`unsupported canonical response reference for ${path}`);
  }
  const name = response.$ref.slice("#/components/schemas/".length);
  const schema = document.components?.schemas?.[name];
  if (!schema) throw new Error(`missing canonical component ${name}`);
  return { name, schema };
}

function requestSchema(path, method = "post") {
  const schema = document.paths?.[path]?.[method]?.requestBody?.content?.["application/json"]?.schema;
  if (!schema) throw new Error(`missing canonical request schema for ${method.toUpperCase()} ${path}`);
  return resolveRef(schema);
}

function resolveRef(schema) {
  if (!schema?.$ref?.startsWith("#/components/schemas/")) return schema;
  const name = schema.$ref.slice("#/components/schemas/".length);
  const resolved = document.components?.schemas?.[name];
  if (!resolved) throw new Error(`missing referenced component ${name}`);
  return resolved;
}

function requireProperties(schema, expected, label) {
  const required = [...(schema.required ?? [])].sort();
  const wanted = [...expected].sort();
  if (JSON.stringify(required) !== JSON.stringify(wanted)) {
    throw new Error(`${label} required properties drifted: ${required.join(",")}`);
  }
}

function enumValues(schema, label) {
  if (!Array.isArray(schema?.enum) || schema.enum.some((value) => typeof value !== "string")) {
    throw new Error(`${label} enum is missing`);
  }
  return schema.enum;
}

const registrationRequest = requestSchema("/v1/auth/register");
requireProperties(registrationRequest, ["first_name", "last_name", "email", "password"], "registration request");
if (registrationRequest.additionalProperties !== false
  || registrationRequest.properties?.first_name?.["x-max-utf8-bytes"] !== 100
  || registrationRequest.properties?.last_name?.["x-max-utf8-bytes"] !== 100) {
  throw new Error("registration request contract drifted");
}

const bootstrap = responseSchema("/v1/auth/session/bootstrap");
const bootstrapData = resolveRef(bootstrap.schema.properties?.data);
requireProperties(bootstrap.schema, ["data", "meta"], bootstrap.name);
requireProperties(bootstrapData, ["csrf_token"], `${bootstrap.name}.data`);

const me = responseSchema("/v1/me");
const meData = resolveRef(me.schema.properties?.data);
requireProperties(me.schema, ["data", "meta"], me.name);
requireProperties(
  meData,
  ["account_type", "portal", "platform_context_generation", "capabilities", "session", "required_actions"],
  `${me.name}.data`,
);
const portalVariants = meData.properties.portal?.oneOf ?? [];
const portalSchema = resolveRef(portalVariants.find((entry) => entry?.$ref));
const sessionSchema = resolveRef(meData.properties.session);
const requiredActionSchema = resolveRef(meData.properties.required_actions?.items);

const accountTypes = enumValues(meData.properties.account_type, "account_type");
const accountTypeSelectionRequest = requestSchema("/v1/me/account-type");
requireProperties(accountTypeSelectionRequest, ["account_type"], "account type selection request");
if (accountTypeSelectionRequest.additionalProperties !== false) {
  throw new Error("account type selection request contract drifted");
}
const selfSelectableAccountTypes = enumValues(accountTypeSelectionRequest.properties?.account_type, "account type selection account_type");
const portalKeys = enumValues(portalSchema.properties.key, "portal.key");
const portalPaths = enumValues(portalSchema.properties.path, "portal.path");
const assuranceLevels = enumValues(sessionSchema.properties.assurance_level, "session.assurance_level");
const requiredActions = enumValues(requiredActionSchema, "required_actions");

const memberships = responseSchema("/v1/me/memberships");
const membershipsData = resolveRef(memberships.schema.properties?.data);
const membershipSchema = resolveRef(membershipsData.properties?.memberships?.items);
requireProperties(memberships.schema, ["data", "meta"], memberships.name);
requireProperties(membershipsData, ["memberships"], `${memberships.name}.data`);
requireProperties(
  membershipSchema,
  ["membership_id", "tenant_id", "tenant_type", "tenant_name", "context_generation", "capabilities"],
  `${memberships.name}.membership`,
);
if (membershipSchema.properties?.tenant_type?.const !== "TENDERPRENEUR") {
  throw new Error("membership tenant_type contract drifted");
}

const sessionInventory = responseSchema("/v1/auth/sessions");
const sessionInventoryData = resolveRef(sessionInventory.schema.properties?.data);
const sessionInventoryItem = resolveRef(sessionInventoryData.properties?.sessions?.items);
requireProperties(sessionInventory.schema, ["data", "meta"], sessionInventory.name);
requireProperties(sessionInventoryData, ["active_session_count", "other_session_count", "sessions", "current_session_logout_available", "other_session_revocation_available", "revoke_all_available", "revocation_policy_configured", "device_metadata_collected", "blockers", "observed_at"], `${sessionInventory.name}.data`);
requireProperties(sessionInventoryItem, ["session_id", "rotated_from_session_id", "current", "created_at", "last_seen_at", "idle_expires_at", "absolute_expires_at"], `${sessionInventory.name}.session`);
if (sessionInventoryData.properties?.device_metadata_collected?.const !== false) {
  throw new Error("session inventory privacy contract drifted");
}

const mfaReadiness = responseSchema("/v1/auth/mfa/preparation");
const mfaReadinessData = resolveRef(mfaReadiness.schema.properties?.data);
const mfaReadinessProperties = [
  "email_verified", "account_disabled", "password_credential_present", "google_identity_present",
  "mfa_enrollment_available", "mfa_enrolled", "step_up_available", "step_up_policy_configured",
  "session_assurance_provenance_available", "recovery_codes_available", "session_authenticated_at",
  "session_age_seconds", "blockers", "observed_at",
];
requireProperties(mfaReadiness.schema, ["data", "meta"], mfaReadiness.name);
requireProperties(mfaReadinessData, mfaReadinessProperties, `${mfaReadiness.name}.data`);

const mfaEnrollment = responseSchema("/v1/auth/mfa/totp/enroll", "post", "201");
const mfaEnrollmentData = resolveRef(mfaEnrollment.schema.properties?.data);
requireProperties(mfaEnrollment.schema, ["data", "meta"], mfaEnrollment.name);
requireProperties(mfaEnrollmentData, ["factor_id", "secret", "otpauth_uri", "expires_at"], `${mfaEnrollment.name}.data`);
const mfaEnrollRequest = requestSchema("/v1/auth/mfa/totp/enroll");
if (mfaEnrollRequest.additionalProperties !== false || mfaEnrollRequest.maxProperties !== 0) {
  throw new Error("MFA enrollment request contract drifted");
}

const mfaActivation = responseSchema("/v1/auth/mfa/totp/confirm", "post", "200");
const mfaActivationData = resolveRef(mfaActivation.schema.properties?.data);
const recoveryCodes = resolveRef(mfaActivationData.properties?.recovery_codes);
requireProperties(mfaActivation.schema, ["data", "meta"], mfaActivation.name);
requireProperties(mfaActivationData, ["recovery_codes", "assurance_level", "assurance_method"], `${mfaActivation.name}.data`);
if (recoveryCodes.minItems !== 10 || recoveryCodes.maxItems !== 10 || mfaActivationData.properties?.assurance_level?.const !== "AAL2" || mfaActivationData.properties?.assurance_method?.const !== "TOTP") {
  throw new Error("MFA activation response contract drifted");
}
const mfaConfirmRequest = requestSchema("/v1/auth/mfa/totp/confirm");
requireProperties(mfaConfirmRequest, ["code"], "MFA confirmation request");
if (mfaConfirmRequest.additionalProperties !== false || mfaConfirmRequest.properties?.code?.pattern !== "^[0-9]{6}$") {
  throw new Error("MFA confirmation request contract drifted");
}

const mfaStepUpRequest = requestSchema("/v1/auth/mfa/step-up");
requireProperties(mfaStepUpRequest, ["code", "method"], "MFA step-up request");
const mfaStepUpMethods = enumValues(mfaStepUpRequest.properties?.method, "MFA step-up method");
if (mfaStepUpRequest.additionalProperties !== false) throw new Error("MFA step-up request contract drifted");
const mfaRecovery = responseSchema("/v1/auth/mfa/recovery-codes/regenerate", "post", "200");
const mfaRecoveryData = resolveRef(mfaRecovery.schema.properties?.data);
const regeneratedCodes = resolveRef(mfaRecoveryData.properties?.recovery_codes);
requireProperties(mfaRecovery.schema, ["data", "meta"], mfaRecovery.name);
requireProperties(mfaRecoveryData, ["recovery_codes"], `${mfaRecovery.name}.data`);
if (regeneratedCodes.minItems !== 10 || regeneratedCodes.maxItems !== 10) throw new Error("MFA recovery response contract drifted");
const mfaReplacementStart = responseSchema("/v1/auth/mfa/totp/replace", "post", "201");
const mfaReplacementComplete = responseSchema("/v1/auth/mfa/totp/replace", "post", "200");
if (resolveRef(mfaReplacementStart.schema.properties?.data).properties?.secret?.pattern !== "^[A-Z2-7]{32}$" || resolveRef(mfaReplacementComplete.schema.properties?.data).properties?.assurance_level?.const !== "AAL2") {
  throw new Error("MFA replacement response contract drifted");
}

const organizationCreationRequest = requestSchema("/v1/tenderpreneur/organizations");
const organizationCreationFields = ["name", "country", "timezone", "locale", "business_type", "primary_industry", "preferred_language", "currency", "commercial_path"];
requireProperties(organizationCreationRequest, organizationCreationFields, "organization creation request");
if (organizationCreationRequest.additionalProperties !== false || organizationCreationRequest.properties?.currency?.const !== "KES") {
  throw new Error("organization creation request contract drifted");
}
const organizationCommercialPaths = enumValues(organizationCreationRequest.properties?.commercial_path, "organization commercial_path");
const organizationCreation = responseSchema("/v1/tenderpreneur/organizations", "post", "202");
const organizationCreationData = resolveRef(organizationCreation.schema.properties?.data);
requireProperties(organizationCreation.schema, ["data", "meta"], organizationCreation.name);
requireProperties(organizationCreationData, ["operation_id", "tenant_id", "state"], `${organizationCreation.name}.data`);
if (organizationCreationData.properties?.state?.const !== "REQUESTED") {
  throw new Error("organization creation response state drifted");
}

const quote = (values) => values.map((value) => JSON.stringify(value)).join(" | ");
const array = (values) => JSON.stringify(values);

const generated = `// Generated from backend/api/openapi/bidora.openapi.json.
// Run \`npm run contracts:generate\`; do not edit by hand.

export interface RegistrationRequest {
  first_name: string;
  last_name: string;
  email: string;
  password: string;
}

export const ACCOUNT_TYPES = ${array(accountTypes)} as const;
export type AccountType = ${quote(accountTypes)};

export const SELF_SELECTABLE_ACCOUNT_TYPES = ${array(selfSelectableAccountTypes)} as const;
export type SelfSelectableAccountType = ${quote(selfSelectableAccountTypes)};

export interface AccountTypeSelectionRequest {
  account_type: SelfSelectableAccountType;
}

export const PORTAL_KEYS = ${array(portalKeys)} as const;
export type PortalKey = ${quote(portalKeys)};

export const PORTAL_PATHS = ${array(portalPaths)} as const;
export type PortalPath = ${quote(portalPaths)};

export const ASSURANCE_LEVELS = ${array(assuranceLevels)} as const;
export type AssuranceLevel = ${quote(assuranceLevels)};

export const REQUIRED_ACTIONS = ${array(requiredActions)} as const;
export type RequiredAction = ${quote(requiredActions)};

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

export const MFA_STEP_UP_METHODS = ${array(mfaStepUpMethods)} as const;
export type MFAStepUpMethod = ${quote(mfaStepUpMethods)};

export interface MFAStepUpRequest {
  code: string;
  method: MFAStepUpMethod;
}

export interface MFARecoveryCodesResponse {
  data: { recovery_codes: string[] };
  meta: { trace_id: string };
}

export const ORGANIZATION_COMMERCIAL_PATHS = ${array(organizationCommercialPaths)} as const;
export type OrganizationCommercialPath = ${quote(organizationCommercialPaths)};

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
`;

if (check) {
  const current = await readFile(outputPath, "utf8").catch(() => "");
  if (current !== generated) {
    console.error("auth contract projection is stale; run npm run contracts:generate");
    process.exit(1);
  }
} else {
  await writeFile(outputPath, generated);
}
