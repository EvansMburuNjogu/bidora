"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { ArrowRightIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { GoogleAuthButton } from "@/components/auth/google-auth-button";
import { PasswordInput } from "@/components/auth/password-input";
import { VerificationCodeInput } from "@/components/auth/verification-code-input";
import { submitAuth, type AuthOperation } from "@/lib/auth-client";
import { isAcceptedEmail, isAcceptedPersonalName, isEmailCode, validatePassword } from "@/lib/auth-validation";

type Mode = Exclude<AuthOperation, "verification-resend" | "login-code-resend">;
type AuthField = "first_name" | "last_name" | "email" | "password" | "confirm_password" | "token" | "code" | "policy";
type AuthFieldError = { field: AuthField; message: string };

const config: Record<Mode, { submit: string }> = {
  login: { submit: "Sign in" },
  register: { submit: "Create account" },
  "password-recovery": { submit: "Send recovery instructions" },
  "password-reset": { submit: "Reset password" },
  "verify-email": { submit: "Verify email" },
  "login-code": { submit: "Verify & Continue" },
};

export function AuthForm({ mode }: { mode: Mode }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<AuthFieldError | null>(null);
  const [termsAccepted, setTermsAccepted] = useState(false);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [verificationAttempt, setVerificationAttempt] = useState(0);
  const [restartPending, setRestartPending] = useState(false);
  const [resendPending, setResendPending] = useState(false);
  const [resendUntil, setResendUntil] = useState(0);
  const [now, setNow] = useState(0);
  const [completed, setCompleted] = useState(false);

  useEffect(() => {
    if (!resendUntil || now >= resendUntil) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [now, resendUntil]);
  const resendSeconds = Math.max(0, Math.ceil((resendUntil - now) / 1000));

  const needsEmail = mode === "login" || mode === "register" || mode === "password-recovery";
  const needsPassword = mode === "login" || mode === "register" || mode === "password-reset";
  const needsToken = mode === "password-reset" || mode === "verify-email";
  const needsCode = mode === "login-code";

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFieldError(null);

    const form = event.currentTarget;
    const data = new FormData(form);
    const firstName = String(data.get("first_name") ?? "");
    const lastName = String(data.get("last_name") ?? "");
    const email = String(data.get("email") ?? "");
    const password = String(data.get("password") ?? "");
    const confirmPassword = String(data.get("confirm_password") ?? "");
    const token = String(data.get("token") ?? "");
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
      const error = validatePassword(password, mode === "login" ? 1 : 12);
      if (error) {
        setFieldError({ field: "password", message: error });
        return;
      }
    }
    if ((mode === "register" || mode === "password-reset") && password !== confirmPassword) {
      setFieldError({ field: "confirm_password", message: "Passwords do not match. Enter the same password again." });
      return;
    }
    if (needsToken && !isEmailCode(token)) {
      setFieldError({ field: "token", message: "Paste the complete code from your Bidora email." });
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
          response = await submitAuth("password-reset", { token, new_password: password });
          break;
        case "verify-email":
          response = await submitAuth("verify-email", { token });
          break;
        case "login-code":
          response = await submitAuth("login-code", { code: normalizedCode, method: "EMAIL_OTP" });
          break;
      }
      if (mode === "login" && response.status === 202) {
        router.push("/auth/login-code");
        return;
      }
      if ((mode === "login" || mode === "login-code") && response.status === 204) {
        if (mode === "login-code") setCompleted(true);
        router.push("/auth/resolve");
        return;
      }
      if (mode === "register" && response.status === 202) {
        form.reset();
        toast.success("Check your email", { description: "Enter the verification code to continue." });
        router.push("/verify-email");
        return;
      }
      if (mode === "password-recovery" && response.status === 202) {
        form.reset();
        toast.success("Check your email", { description: "Enter the recovery code to reset your password." });
        router.push("/reset-password");
        return;
      }
      if ((mode === "password-reset" || mode === "verify-email") && response.status === 204) {
        form.reset();
        toast.success(mode === "password-reset" ? "Password reset" : "Email verified");
        router.push("/login");
        return;
      }
      if (mode === "login-code" && response.status === 401) {
        form.reset();
        setVerificationAttempt((value) => value + 1);
        setFieldError({ field: "code", message: "Invalid or expired code. Check the latest email or request a new code." });
        return;
      }
      toast.error(response.status === 429 ? "Try again later" : "Unable to continue", {
        description: response.status === 429 ? "Please wait before trying again." : "Check what you entered and try again.",
      });
    } catch {
      toast.error("Bidora is unavailable", { description: "Nothing was changed. Please try again." });
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
      } else {
        toast.error(response.status === 401 ? "Sign in again to request a code" : "Couldn’t send a new code");
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
              <FieldLabel htmlFor={`${mode}-email`}>{mode === "login" ? "Work email" : mode === "register" ? "Work email address" : "Email address"}</FieldLabel>
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
              {fieldError?.field === "email" && <FieldError>{fieldError.message}</FieldError>}
            </Field>
          )}

          {needsToken && (
            <Field data-invalid={fieldError?.field === "token"}>
              <FieldLabel htmlFor={`${mode}-token`}>{mode === "password-reset" ? "Recovery code" : "Verification code"}</FieldLabel>
              <Input
                id={`${mode}-token`}
                name="token"
                type="text"
                autoComplete="one-time-code"
                placeholder="Paste the code from your email"
                minLength={43}
                maxLength={43}
                disabled={pending}
                required
                aria-invalid={fieldError?.field === "token" || undefined}
                onChange={() => fieldError?.field === "token" && setFieldError(null)}
              />
              <FieldDescription>Copy and paste the full code exactly as it appears in the email.</FieldDescription>
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
                  <Link href="/forgot-password" className="type-meta text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
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
              {mode !== "login" && <FieldDescription>Use a long passphrase (12–128 bytes).</FieldDescription>}
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
              <FieldDescription>Use the current code once, then continue.</FieldDescription>
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

          <Button type="submit" size="lg" className="w-full" disabled={pending || restartPending || resendPending || completed || (mode === "register" && (!termsAccepted || !privacyAccepted))} aria-describedby={mode === "register" ? "registration-policy-status" : undefined}>
            {completed ? "Verified. Continuing…" : pending ? "Verifying…" : config[mode].submit}
            {mode === "login" && !pending && <ArrowRightIcon aria-hidden="true" />}
          </Button>
        </FieldGroup>
      </form>

      {mode === "login-code" && (
        <div className="flex flex-col items-center gap-2 text-center text-sm">
          <p className="text-muted-foreground">Codes expire after five minutes. Use the latest email.</p>
          <Button type="button" variant="outline" size="sm" disabled={pending || restartPending || resendPending || completed || resendSeconds > 0} onClick={() => void resendLoginCode()}>
            {resendPending ? "Sending…" : resendSeconds > 0 ? `Resend in ${resendSeconds}s` : "Resend code"}
          </Button>
          <Button type="button" variant="ghost" size="sm" disabled={pending || restartPending || resendPending || completed} onClick={() => void restartLogin()}>
            {restartPending ? "Returning…" : "Back to sign in"}
          </Button>
        </div>
      )}

      {(mode === "login" || mode === "register") && (
        <GoogleAuthButton registration={mode === "register"} disabled={mode === "register" && (!termsAccepted || !privacyAccepted)} />
      )}

      {mode === "verify-email" && <VerificationResendForm />}

      <AuthRouteLink
        mode={mode}
        onNavigate={() => {
          setFieldError(null);
          if (mode === "login-code") setVerificationAttempt((value) => value + 1);
        }}
      />
    </div>
  );
}

function VerificationResendForm() {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function resend(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = event.currentTarget;
    const email = String(new FormData(form).get("email") ?? "");
    if (!isAcceptedEmail(email)) {
      setError("Enter a valid email address.");
      return;
    }
    setPending(true);
    try {
      const response = await submitAuth("verification-resend", { email });
      if (response.status === 202) {
        form.reset();
        toast.success("Check your email", { description: "If verification is available, a replacement code has been sent." });
        return;
      }
      toast.error(response.status === 429 ? "Try again later" : "Unable to resend", {
        description: response.status === 429 ? "Please wait before trying again." : "Nothing was changed. Please try again.",
      });
    } catch {
      toast.error("Bidora is unavailable", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={resend} noValidate className="rounded-lg border border-border p-4">
      <FieldGroup className="gap-3">
        <Field data-invalid={Boolean(error)}>
          <FieldLabel htmlFor="verification-resend-email">Need a new code?</FieldLabel>
          <Input
            id="verification-resend-email"
            name="email"
            type="email"
            autoComplete="email"
            placeholder="name@company.com"
            maxLength={254}
            disabled={pending}
            required
            aria-invalid={Boolean(error) || undefined}
            onChange={() => error && setError("")}
          />
          {error && <FieldError>{error}</FieldError>}
        </Field>
        <Button type="submit" variant="outline" className="w-full" disabled={pending}>
          {pending ? "Sending…" : "Send replacement code"}
        </Button>
      </FieldGroup>
    </form>
  );
}

function AuthRouteLink({ mode, onNavigate }: { mode: Mode; onNavigate: () => void }) {
  if (mode === "login") {
    return <p className="type-body text-center text-muted-foreground">New to Bidora? <Link href="/register" onClick={onNavigate} className="font-medium text-foreground underline-offset-4 hover:underline">Create an account</Link></p>;
  }
  if (mode === "register") {
    return <p className="type-body text-center text-muted-foreground">Already have an account? <Link href="/login" onClick={onNavigate} className="font-medium text-foreground underline-offset-4 hover:underline">Sign in</Link></p>;
  }
  if (mode === "login-code") return null;
  return <p className="type-body text-center text-muted-foreground"><Link href="/login" onClick={onNavigate} className="font-medium text-foreground underline-offset-4 hover:underline">Back to sign in</Link></p>;
}
