"use client";

import Link from "next/link";
import { useRouter,useSearchParams } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { ArrowLeftIcon, ArrowRightIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { GoogleAuthButton } from "@/components/auth/google-auth-button";
import { PasswordInput } from "@/components/auth/password-input";
import { usePasswordPolicy } from "@/components/auth/use-password-policy";
import { VerificationCodeInput } from "@/components/auth/verification-code-input";
import { useLoginChallenge } from "@/components/auth/login-challenge-provider";
import { challengeDeadline, countdownLabel, maskLoginEmail } from "@/lib/login-challenge";
import { submitAuth, type AuthOperation } from "@/lib/auth-client";
import {withInvitation,invitationNext} from "@/lib/invitation-code";
import { emailCodeQuery, emailCodeReference } from "@/lib/email-code-challenge";
import { isAcceptedEmail, isAcceptedPersonalName, isEmailCode, validatePassword } from "@/lib/auth-validation";

type Mode = Exclude<AuthOperation, "verification-resend" | "login-code-resend">;
type AuthField = "first_name" | "last_name" | "email" | "password" | "confirm_password" | "token" | "code" | "policy";
type AuthFieldError = { field: AuthField; message: string; cooldown?: boolean };

const config: Record<Mode, { submit: string }> = {
  login: { submit: "Sign in" },
  register: { submit: "Create account" },
  "password-recovery": { submit: "Send recovery link" },
  "password-reset": { submit: "Reset password" },
  "verify-email": { submit: "Verify email" },
  "login-code": { submit: "Verify & Continue" },
};

export function AuthForm({ mode, recoveryUserId }: { mode: Mode; recoveryUserId?: string }) {
 const flow=emailCodeReference(useSearchParams().get("invitation"));
  const recoveryTokenInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (mode !== "password-reset" || !recoveryUserId) return;
    const code = new URLSearchParams(window.location.hash.slice(1)).get("code") ?? "";
    if (isEmailCode(code) && recoveryTokenInput.current) recoveryTokenInput.current.value = code;
    // Never retain the credential in browser history or send it to the server.
    if (window.location.hash) window.history.replaceState(window.history.state, "", window.location.pathname + window.location.search);
  }, [mode, recoveryUserId]);
  const router = useRouter();
  const passwordPolicy = usePasswordPolicy(mode === "register" || mode === "password-reset");
  const { hint, setHint } = useLoginChallenge();
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<AuthFieldError | null>(null);
  const [termsAccepted, setTermsAccepted] = useState(false);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [verificationAttempt, setVerificationAttempt] = useState(0);
  const [restartPending, setRestartPending] = useState(false);
  const [resendPending, setResendPending] = useState(false);
  const [resendExpired, setResendExpired] = useState(false);
  const [resendUntil, setResendUntil] = useState(0);
  const [now, setNow] = useState(0);
  const [completed, setCompleted] = useState(false);
  const [continuationUnavailable, setContinuationUnavailable] = useState(false);

  useEffect(() => {
    if (mode !== "login-code" && mode !== "password-recovery") return;
    const timer = window.setInterval(() => {
      const at = Date.now();
      setNow(at);
      if (mode === "password-recovery" && resendUntil > 0 && at >= resendUntil) {
        setResendUntil(0);
        setFieldError(current => current?.cooldown ? null : current);
      }
    }, 1000);
    return () => window.clearInterval(timer);
  }, [mode, resendUntil]);
  const resendSeconds = Math.max(0, Math.ceil((resendUntil - now) / 1000));
  const challengeExpired = mode === "login-code" && hint.expiresAt !== null && now > 0 && now >= hint.expiresAt;

  const needsEmail = mode === "login" || mode === "register" || mode === "password-recovery";
  const needsPassword = mode === "login" || mode === "register" || mode === "password-reset";
  const needsToken = mode === "password-reset" || mode === "verify-email";
  const needsCode = mode === "login-code";

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || (mode === "password-recovery" && resendSeconds > 0) || ((mode === "register" || mode === "password-reset") && passwordPolicy.minimum === null)) return;
    setFieldError(null);

    const form = event.currentTarget;
    const data = new FormData(form);
    const firstName = String(data.get("first_name") ?? "");
    const lastName = String(data.get("last_name") ?? "");
    const email = String(data.get("email") ?? "");
    const password = String(data.get("password") ?? "");
    const confirmPassword = String(data.get("confirm_password") ?? "");
    const enteredCode = String(data.get("code") ?? "").trim();
    const token = /^\d{6}$/.test(enteredCode) ? enteredCode : String(data.get("token") || enteredCode || "").trim();
    const challengeRef = emailCodeReference(new URLSearchParams(window.location.search).get("challenge_ref"));
    const code = String(data.get("code") ?? "");
    const normalizedCode = code.trim().toUpperCase();

    if (mode === "register" && (!termsAccepted || !privacyAccepted)) {
      setFieldError({ field: "policy", message: "Accept the Terms and Conditions and Privacy Policy to continue." });
      return;
    }
    if (mode === "register" && !isAcceptedPersonalName(firstName)) {
      setFieldError({ field: "first_name", message: "Enter a valid first name." });
      return;
    }
    if (mode === "register" && !isAcceptedPersonalName(lastName)) {
      setFieldError({ field: "last_name", message: "Enter a valid last name." });
      return;
    }
    if (needsEmail && !isAcceptedEmail(email)) {
      setFieldError({ field: "email", message: "Enter a valid email address." });
      return;
    }
    if (needsPassword) {
      const error = validatePassword(password, mode === "login" ? 1 : passwordPolicy.minimum!);
      if (error) {
        setFieldError({ field: "password", message: error });
        return;
      }
    }
    if ((mode === "register" || mode === "password-reset") && password !== confirmPassword) {
      setFieldError({ field: "confirm_password", message: "Passwords do not match. Enter the same password again." });
      return;
    }
    if (needsToken && !isEmailCode(token) && !/^\d{6}$/.test(token)) {
      setFieldError({ field: "token", message: "Enter the six-digit code from your email." });
      return;
    }
    if (needsToken && /^\d{6}$/.test(token) && !challengeRef) {
      setFieldError({ field: "token", message: "Open the latest email link or request a new code." });
      return;
    }
    if (needsCode && !/^\d{6}$/.test(normalizedCode)) {
      setFieldError({ field: "code", message: "Enter the complete six-digit verification code." });
      return;
    }

    setPending(true);
    try {
      let response: Response;
      switch (mode) {
        case "login":
          response = await submitAuth("login", { email, password });
          break;
        case "register":
          response = await submitAuth("register", { first_name: firstName, last_name: lastName, email, password });
          break;
        case "password-recovery":
          response = await submitAuth("password-recovery", { email });
          break;
        case "password-reset":
          response = await submitAuth("password-reset", { token, new_password: password, ...(challengeRef ? { challenge_ref: challengeRef } : {}), ...(recoveryUserId ? { user_id: recoveryUserId } : {}) });
          break;
        case "verify-email":
          response = await submitAuth("verify-email", { token, ...(challengeRef ? { challenge_ref: challengeRef } : {}) });
          break;
        case "login-code":
          response = await submitAuth("login-code", { code: normalizedCode, method: "EMAIL_OTP" });
          break;
      }
      if (mode === "login" && response.status === 202) {
        let expiresAt = challengeDeadline(response.headers.get("X-Bidora-Challenge-Expires-At"));
        if (expiresAt === null) {
          const body = await response.json().catch(() => null);
          expiresAt = challengeDeadline(body?.data?.expires_at);
        }
        setHint({ maskedEmail: maskLoginEmail(email), expiresAt });
        form.reset();
        router.push(withInvitation("/auth/login-code",flow));
        return;
      }
      if ((mode === "login" || mode === "login-code") && response.status === 204) {
        if (mode === "login-code") setCompleted(true);
        setHint({ maskedEmail: null, expiresAt: null });
        const next=invitationNext(response.headers.get("X-Bidora-Invitation-Next"));
        if(next)window.location.assign(next);
        else router.push("/auth/resolve");
        return;
      }
      if (mode === "register" && response.status === 202) {
        form.reset();
        toast.info("Check your email", { description: "If your account can be registered, check your inbox for a verification code. If you already have an account, sign in or request a new code." });
        router.push(withInvitation("/verify-email" + emailCodeQuery(response),flow));
        return;
      }
      if (mode === "password-recovery" && response.status === 202) {
        form.reset();
        const query = emailCodeQuery(response);
        router.replace(withInvitation(query ? "/reset-password" + query : "/forgot-password/check-email",flow));
        return;
      }
      if ((mode === "password-reset" || mode === "verify-email") && response.status === 204) {
        form.reset();
        const continued = mode === "verify-email" && response.headers.get("X-Bidora-Registration-Session") === "1";
        const next=invitationNext(response.headers.get("X-Bidora-Invitation-Next"));
        if (mode === "verify-email" && !continued && !next) {
          setCompleted(true);
          setContinuationUnavailable(true);
          return;
        }
        toast.success(mode === "password-reset" ? "Password reset" : "Email verified", { description: continued ? "Continue setting up your account." : "Sign in to continue." });
        // Full navigation prevents a prefetched unauthenticated session from being reused.
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination
        if (continued) window.location.assign("/organization/create");
        else if(mode==="verify-email"&&next)window.location.assign(next);
        else router.push(withInvitation("/login",flow));
        return;
      }
      if (mode === "verify-email" && response.status === 400) {
        setVerificationAttempt(value => value + 1);
        setFieldError({ field: "token", message: "Invalid or expired code. Check your email or resend a code." });
        return;
      }
      if (mode === "verify-email" && response.status === 503) {
        const body = await response.json().catch(() => null);
        if (body?.error?.code === "REGISTRATION_CONTINUATION_UNAVAILABLE") {
          setContinuationUnavailable(true);
          setCompleted(true);
          return;
        }
        toast.error("Verification could not be confirmed", { description: "Try signing in. If your email is not verified, request a new code." });
        return;
      }
      if ((mode === "register" || mode === "password-reset") && response.status === 400) passwordPolicy.retry();
      if (mode === "password-reset" && response.status === 400) {
        setFieldError({ field: "token", message: "The recovery code or password requirements are invalid. Check the latest email and requirements, then try again." });
        return;
      }
      if (mode === "password-reset" && response.status === 503) {
        toast.error("Password reset could not be confirmed", { description: "Try signing in with your new password. If you still cannot sign in, request a new recovery email." });
        return;
      }
      if (mode === "login-code" && response.status === 401) {
        form.reset();
        setVerificationAttempt((value) => value + 1);
        setFieldError({ field: "code", message: "Invalid or expired code. Check the latest email or request a new code." });
        return;
      }
      if (mode === "password-recovery" && response.status === 429) {
        const retry = Number(response.headers.get("Retry-After"));
        // eslint-disable-next-line react-hooks/purity -- This runs only in the async form-submit handler after a 429, never during render.
        const at=Date.now();
        setNow(at);
        setResendUntil(at + (Number.isFinite(retry) && retry > 0 && retry <= 3600 ? retry : 900) * 1000);
        setFieldError({ field: "email", message: "Too many recovery requests. Wait before requesting another link.", cooldown: true });
        return;
      }
      if (mode === "password-recovery" && response.status === 503) {
        toast.error("Recovery email unavailable", { description: "Email delivery is not ready. Contact your administrator or try again later." });
        return;
      }
      toast.error(response.status === 429 ? "Try again later" : "Unable to continue", {
        description: response.status === 429 ? "Please wait before trying again." : "Check what you entered and try again.",
      });
    } catch {
      toast.error("Bidora is unavailable", { description: "The request could not be confirmed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  async function resendLoginCode() {
    if (resendPending || resendSeconds > 0) return;
    setResendPending(true);
    try {
      const response = await submitAuth("login-code-resend", {});
      if (response.status === 202) {
        setResendExpired(false);
        setHint({ ...hint, expiresAt: challengeDeadline(response.headers.get("X-Bidora-Challenge-Expires-At")) });
        setVerificationAttempt((value) => value + 1);
        setFieldError(null);
        setNow(Date.now());
        setResendUntil(Date.now() + 30_000);
        toast.success("New code sent");
      } else if (response.status === 429) {
        const retry = Number(response.headers.get("Retry-After"));
        if (Number.isFinite(retry) && retry > 0 && retry <= 900) {
          setNow(Date.now());
          setResendUntil(Date.now() + retry * 1000);
        }
        toast.error("Please wait before requesting another code");
      } else if (response.status === 401) {
        setResendExpired(true);
        setHint({ ...hint, expiresAt: Date.now() });
        setNow(Date.now());
        toast.error("Sign-in expired", { description: "Back to sign in to request a new code." });
      } else {
        toast.error("Couldn’t send a new code", { description: response.status === 503 ? "Email delivery is unavailable. Try again later." : "Please try again." });
      }
    } catch {
      toast.error("Couldn’t send a new code");
    } finally {
      setResendPending(false);
    }
  }

  async function restartLogin() {
    setRestartPending(true);
    try {
      const response = await fetch("/api/auth/login-code/cancel", { method: "POST", credentials: "include", cache: "no-store" });
      if (response.status === 204) {
        window.location.replace("/login");
        return;
      }
      toast.error("Unable to return to sign in");
    } catch {
      toast.error("Unable to return to sign in");
    } finally {
      setRestartPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <form onSubmit={onSubmit} noValidate>
        <FieldGroup className={mode === "login" || mode === "login-code" ? "gap-4" : undefined}>
          {mode === "register" && (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field data-invalid={fieldError?.field === "first_name"}>
                <FieldLabel htmlFor="register-first-name">First name</FieldLabel>
                <Input
                  id="register-first-name"
                  name="first_name"
                  type="text"
                  autoComplete="given-name"
                  placeholder="First name"
                  maxLength={100}
                  disabled={pending}
                  required
                  autoFocus
                  className="h-10 px-3"
                  aria-invalid={fieldError?.field === "first_name" || undefined}
                  onChange={() => fieldError?.field === "first_name" && setFieldError(null)}
                />
                {fieldError?.field === "first_name" && <FieldError>{fieldError.message}</FieldError>}
              </Field>
              <Field data-invalid={fieldError?.field === "last_name"}>
                <FieldLabel htmlFor="register-last-name">Last name</FieldLabel>
                <Input
                  id="register-last-name"
                  name="last_name"
                  type="text"
                  autoComplete="family-name"
                  placeholder="Last name"
                  maxLength={100}
                  disabled={pending}
                  required
                  className="h-10 px-3"
                  aria-invalid={fieldError?.field === "last_name" || undefined}
                  onChange={() => fieldError?.field === "last_name" && setFieldError(null)}
                />
                {fieldError?.field === "last_name" && <FieldError>{fieldError.message}</FieldError>}
              </Field>
            </div>
          )}

          {needsEmail && (
            <Field data-invalid={fieldError?.field === "email"}>
              <FieldLabel htmlFor={`${mode}-email`}>{mode === "login" ? "Work email" : "Email address"}</FieldLabel>
              <Input
                id={`${mode}-email`}
                name="email"
                type="email"
                autoComplete="email"
                placeholder="name@company.com"
                maxLength={254}
                disabled={pending}
                required
                className={mode === "register" ? "h-10 px-3" : undefined}
                aria-invalid={fieldError?.field === "email" || undefined}
                onChange={() => fieldError?.field === "email" && setFieldError(null)}
              />
              {mode === "register" && <FieldDescription>Use an email address you can access. A work email is recommended.</FieldDescription>}
              {fieldError?.field === "email" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {needsToken && (
            <Field data-invalid={fieldError?.field === "token"}>
              <FieldLabel>{mode === "password-reset" ? "Recovery code" : "Verification code"}</FieldLabel>
              <input ref={recoveryTokenInput} name="token" type="hidden" />
              <VerificationCodeInput key={verificationAttempt} disabled={pending} invalid={fieldError?.field === "token"} allowAutoComplete={verificationAttempt === 0} onEdit={() => { if (recoveryTokenInput.current) recoveryTokenInput.current.value = ""; if (fieldError?.field === "token") setFieldError(null); }} onLegacyPaste={value => { if (recoveryTokenInput.current) recoveryTokenInput.current.value = value; toast.message("Code added from your email."); }} />
              {fieldError?.field === "token" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {needsPassword && (
            <Field data-invalid={fieldError?.field === "password"}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <FieldLabel htmlFor={`${mode}-password`}>
                  {mode === "password-reset" ? "New password" : "Password"}
                </FieldLabel>
                {mode === "login" && (
                  <Link href={withInvitation("/forgot-password",flow)} className="type-meta text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
                    Forgot password?
                  </Link>
                )}
              </div>
              <PasswordInput
                id={`${mode}-password`}
                name="password"
                autoComplete={mode === "login" ? "current-password" : "new-password"}
                placeholder={mode === "login" ? "Enter your password" : mode === "password-reset" ? "Create a new password" : "Create a password"}
                maxLength={128}
                disabled={pending}
                required
                className={mode === "register" ? "h-10 px-3 pe-10" : undefined}
                aria-invalid={fieldError?.field === "password" || undefined}
                onChange={() => fieldError?.field === "password" && setFieldError(null)}
              />
              {mode !== "login" && <FieldDescription>{passwordPolicy.minimum === null ? passwordPolicy.failed ? "Password requirements are unavailable." : "Loading password requirements…" : `Use a long passphrase (${passwordPolicy.minimum}–128 bytes).`}</FieldDescription>}
              {mode !== "login" && passwordPolicy.failed && <Button type="button" variant="outline" size="sm" onClick={passwordPolicy.retry}>Retry password requirements</Button>}
              {fieldError?.field === "password" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {(mode === "register" || mode === "password-reset") && (
            <Field data-invalid={fieldError?.field === "confirm_password"}>
              <FieldLabel htmlFor={`${mode}-confirm-password`}>Confirm {mode === "password-reset" ? "new " : ""}password</FieldLabel>
              <PasswordInput
                id={`${mode}-confirm-password`}
                name="confirm_password"
                autoComplete="new-password"
                placeholder="Enter the same password again"
                maxLength={128}
                disabled={pending}
                required
                className={mode === "register" ? "h-10 px-3 pe-10" : undefined}
                aria-invalid={fieldError?.field === "confirm_password" || undefined}
                onChange={() => fieldError?.field === "confirm_password" && setFieldError(null)}
              />
              {fieldError?.field === "confirm_password" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {needsCode && (
            <Field data-invalid={fieldError?.field === "code"}>
              <FieldLabel>Verification code</FieldLabel>
              <VerificationCodeInput
                key={verificationAttempt}
                disabled={pending || restartPending}
                invalid={fieldError?.field === "code"}
                allowAutoComplete={verificationAttempt === 0}
                onEdit={() => { if (fieldError?.field === "code") setFieldError(null); }}
              />
              {fieldError?.field === "code" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {mode === "register" && (
            <Field id="registration-policy-status" data-invalid={fieldError?.field === "policy"} className="space-y-3">
              <FieldLabel className="w-full rounded-lg border bg-card p-3">
                <Checkbox checked={termsAccepted} onCheckedChange={(checked) => { setTermsAccepted(checked); if (fieldError?.field === "policy") setFieldError(null); }} aria-invalid={fieldError?.field === "policy" || undefined} />
                <span className="text-sm font-normal">I agree to Bidora’s <Link href="/legal/terms" className="font-medium text-foreground underline underline-offset-4">Terms and Conditions</Link>.</span>
              </FieldLabel>
              <FieldLabel className="w-full rounded-lg border bg-card p-3">
                <Checkbox checked={privacyAccepted} onCheckedChange={(checked) => { setPrivacyAccepted(checked); if (fieldError?.field === "policy") setFieldError(null); }} aria-invalid={fieldError?.field === "policy" || undefined} />
                <span className="text-sm font-normal">I agree to Bidora’s <Link href="/legal/privacy" className="font-medium text-foreground underline underline-offset-4">Privacy Policy</Link>.</span>
              </FieldLabel>
              {fieldError?.field === "policy" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          <Button type="submit" size="lg" className="w-full" disabled={((mode === "register" || mode === "password-reset") && passwordPolicy.minimum === null) || pending || restartPending || resendPending || completed || challengeExpired || (mode === "password-recovery" && resendSeconds > 0) || (mode === "register" && (!termsAccepted || !privacyAccepted))} aria-describedby={mode === "register" ? "registration-policy-status" : undefined}>
            {continuationUnavailable ? "Email verified" : completed ? "Verified. Continuing…" : pending ? mode === "register" ? "Creating account…" : mode === "password-recovery" ? "Sending…" : "Verifying…" : mode === "password-recovery" && resendSeconds > 0 ? `Try again in ${countdownLabel(resendUntil, now)}` : config[mode].submit}
            {mode === "login" && !pending && <ArrowRightIcon aria-hidden="true" />}
          </Button>
        </FieldGroup>
      </form>

      {mode === "login-code" && (
        <div className="flex flex-col items-center gap-2 text-center text-sm">
          {hint.expiresAt !== null && now > 0 ? (
            <p role="timer" aria-live="off" className="text-muted-foreground tabular-nums">
              {resendExpired ? "Sign-in expired. Sign in again for a new code." : challengeExpired ? "Code expired. Resend for a new code." : `Expires in ${countdownLabel(hint.expiresAt, now)}`}
            </p>
          ) : <p className="text-muted-foreground">Use your latest code.</p>}
          <Button type="button" variant="outline" size="sm" disabled={pending || restartPending || resendPending || completed || resendExpired || resendSeconds > 0} onClick={() => void resendLoginCode()}>
            {resendPending ? "Sending…" : resendSeconds > 0 ? `Resend in ${resendSeconds}s` : "Resend code"}
          </Button>
          <Button type="button" variant="ghost" size="sm" disabled={pending || restartPending || resendPending || completed} onClick={() => void restartLogin()}>
            <ArrowLeftIcon aria-hidden="true" className="size-4" />{restartPending ? "Returning…" : "Back to sign in"}
          </Button>
        </div>
      )}

      {(mode === "login" || mode === "register") && (
        <GoogleAuthButton registration={mode === "register"} disabled={mode === "register" && (!termsAccepted || !privacyAccepted)} />
      )}

      {continuationUnavailable && <p role="alert" className="text-center text-sm text-muted-foreground">Your email is verified, but we could not confirm your session. <Link href={withInvitation("/login",flow)} className="font-medium text-foreground underline">Sign in to continue</Link>.</p>}
      {(mode==="verify-email"||mode==="password-reset")&&!continuationUnavailable&&<VerificationResendForm mode={mode} onResent={()=>{if(recoveryTokenInput.current)recoveryTokenInput.current.value="";setVerificationAttempt(value=>value+1);setFieldError(null)}}/>}

      {!continuationUnavailable && <AuthRouteLink
        mode={mode}
        flow={flow}
        onNavigate={() => {
          setFieldError(null);
          if (mode === "login-code") setVerificationAttempt((value) => value + 1);
        }}
      />}
    </div>
  );
}

function VerificationResendForm({mode,onResent}:{mode:"verify-email"|"password-reset";onResent:()=>void}) {
 const router=useRouter();
 const [pending,setPending]=useState(false),[error,setError]=useState("");
 const [countdown,setCountdown]=useState(mode==="verify-email"?30:0);
 const locked=useRef(false),resendAt=useRef(0);
 useEffect(()=>{
  if(mode!=="verify-email")return;
  resendAt.current=Date.now()+30_000;
  const timer=window.setInterval(()=>setCountdown(Math.max(0,Math.ceil((resendAt.current-Date.now())/1000))),1000);
  return()=>window.clearInterval(timer);
 },[mode]);
 async function resend(){
  if(locked.current||countdown>0)return;
  const challenge_ref=emailCodeReference(new URLSearchParams(window.location.search).get("challenge_ref"));
  if(!challenge_ref){setError("Verification context is unavailable. Return to registration or reopen your invitation to request a new code.");return;}
  locked.current=true;setPending(true);setError("");
  try{
   const response=await submitAuth(mode==="password-reset"?"password-recovery":"verification-resend",{challenge_ref});
   if(response.status===202){
    const query=emailCodeQuery(response);
    if(mode==="verify-email"&&!query){resendAt.current=Date.now()+30_000;setCountdown(30);setError("The new code request could not be confirmed. Please try again later.");return;}
    if(query)router.replace(withInvitation((mode==="password-reset"?"/reset-password":"/verify-email")+query,emailCodeReference(new URLSearchParams(window.location.search).get("invitation"))));
    if(mode==="verify-email"){resendAt.current=Date.now()+30_000;setCountdown(30);}
    onResent();toast.info("Code requested",{description:"If the account is eligible, check your email for the latest code."});return;
   }
   if(response.status===429&&mode==="verify-email"){
    const retry=Number(response.headers.get("Retry-After"));
    if(Number.isFinite(retry)&&retry>0&&retry<=3600){resendAt.current=Date.now()+retry*1000;setCountdown(Math.ceil(retry));return;}
   }
   setError(response.status===429?"Please try again later.":response.status===400?"Verification context expired. Return to registration or reopen your invitation.":"Unable to request a code. Please try again.");
  }catch{setError("The request could not be confirmed. Please wait before trying again.")}
  finally{locked.current=false;setPending(false)}
 }
 return <div className="flex flex-col items-center gap-2">
  {mode==="verify-email"&&countdown>0&&<p role="timer" aria-live="off" className="text-sm text-muted-foreground tabular-nums">You can resend a code in {countdownLabel(countdown*1000,0)}</p>}
  <Button type="button" variant="outline" size="sm" disabled={pending||countdown>0} onClick={()=>void resend()}>{pending?"Sending…":"Resend code"}</Button>
  {error&&<p role="alert" className="type-meta text-destructive">{error}</p>}
 </div>;
}

function AuthRouteLink({ mode, flow,onNavigate }: { mode: Mode;flow:string|null; onNavigate: () => void }) {
  if (mode === "login") {
    return <p className="type-body text-center text-muted-foreground">New to Bidora? <Link href={withInvitation("/register",flow)} onClick={onNavigate} className="font-medium text-foreground underline-offset-4 hover:underline">Create an account</Link></p>;
  }
  if (mode === "register") {
    return <p className="type-body text-center text-muted-foreground">Already have an account? <Link href={withInvitation("/login",flow)} onClick={onNavigate} className="font-medium text-foreground underline-offset-4 hover:underline">Sign in</Link></p>;
  }
  if (mode === "login-code") return null;
  if (mode === "verify-email") return <p className="type-body text-center text-muted-foreground"><Link href={withInvitation("/register",flow)} onClick={onNavigate} className="inline-flex items-center gap-2 font-medium text-foreground underline-offset-4 hover:underline"><ArrowLeftIcon aria-hidden="true" className="size-4" />Back to registration</Link></p>;
  return <p className="type-body text-center text-muted-foreground"><Link href={withInvitation("/login",flow)} onClick={onNavigate} className="inline-flex items-center gap-2 font-medium text-foreground underline-offset-4 hover:underline"><ArrowLeftIcon aria-hidden="true" className="size-4" />Back to sign in</Link></p>;
}
