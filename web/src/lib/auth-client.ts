import {emailCodeReference} from "@/lib/email-code-challenge";
import { announceSessionChange } from "@/lib/session-events";
import type { MFAConfirmRequest, MFAEnrollRequest, MFAStepUpRequest, RegistrationRequest } from "@/contracts/auth.generated";

export interface AuthPayloads {
  login: { email: string; password: string };
  register: RegistrationRequest;
  "password-recovery": { email?: string;challenge_ref?:string };
  "password-reset": { token: string; new_password: string; user_id?: string; challenge_ref?: string };
  "verify-email": { token: string; challenge_ref?: string };
  "verification-resend": { email?: string;challenge_ref?:string };
  "login-code": { code: string; method: "EMAIL_OTP" };
  "login-code-resend": Record<string, never>;
}

export type AuthOperation = keyof AuthPayloads;

export async function submitAuth<Operation extends AuthOperation>(
  operation: Operation,
  payload: AuthPayloads[Operation],
): Promise<Response> {
  const response = await fetch(`/api/auth/${operation}`, {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: { "Content-Type": "application/json",...(typeof window!=="undefined"&&emailCodeReference(new URLSearchParams(window.location.search).get("invitation"))?{"X-Bidora-Invitation-Flow":new URLSearchParams(window.location.search).get("invitation")!}:{}) },
    body: JSON.stringify(payload),
  });
  if (response.status === 204 && ["login", "login-code", "password-reset", "verify-email"].includes(operation)) announceSessionChange();
  return response;
}

export async function beginMFAEnrollment(payload: MFAEnrollRequest, csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-enroll", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(payload),
  });
}

export async function confirmMFAEnrollment(payload: MFAConfirmRequest, csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-confirm", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(payload),
  });
}

export async function stepUpMFA(payload: MFAStepUpRequest, csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-step-up", {
    method: "POST", credentials: "include", cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(payload),
  });
}

export async function regenerateMFARecoveryCodes(csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-recovery-codes", {
    method: "POST", credentials: "include", cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: "{}",
  });
}

export async function beginMFAReplacement(csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-replace", {
    method: "POST", credentials: "include", cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: "{}",
  });
}

export async function confirmMFAReplacement(payload: MFAConfirmRequest, csrfToken: string): Promise<Response> {
  return fetch("/api/auth/mfa-replace", {
    method: "POST", credentials: "include", cache: "no-store",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(payload),
  });
}

export type PasswordChangePayload = {
  email: string;
  current_password: string;
  new_password: string;
};

export async function submitPasswordChange(payload: PasswordChangePayload, csrfToken: string): Promise<Response> {
  return fetch("/api/auth/password-change", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrfToken,
    },
    body: JSON.stringify(payload),
  });
}

export async function submitLogout(csrfToken: string): Promise<Response> {
  return fetch("/api/auth/logout", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: { "X-CSRF-Token": csrfToken },
  });
}

export function byteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}
