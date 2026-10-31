"use client";

import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { GoogleIcon } from "@/components/auth/google-icon";
import { PasswordInput } from "@/components/auth/password-input";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import { isAcceptedEmail } from "@/lib/auth-validation";
import { parseMFAReadiness } from "@/lib/mfa-contract";

type Status = "loading" | "available" | "linked" | "error";

export function GoogleLinking() {
  const { csrfToken } = useSession();
  const [status, setStatus] = useState<Status>("loading");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const response = await fetch("/api/auth/mfa/preparation", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal: controller.signal });
        if (!response.ok) return setStatus("error");
        const readiness = parseMFAReadiness(await response.json());
        setStatus(readiness ? readiness.data.google_identity_present ? "linked" : readiness.data.password_credential_present ? "available" : "error" : "error");
      } catch (cause) {
        if (!(cause instanceof DOMException && cause.name === "AbortError")) setStatus("error");
      }
    }
    void load();
    return () => controller.abort();
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const data = new FormData(event.currentTarget);
    const email = String(data.get("email") ?? "");
    const password = String(data.get("password") ?? "");
    if (!isAcceptedEmail(email) || password.length < 1) {
      setError("Enter this account’s email and current password.");
      return;
    }
    setPending(true);
    try {
      const response = await fetch("/api/auth/google-link/start", {
        method: "POST", credentials: "include", cache: "no-store",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
        body: JSON.stringify({ email, password }),
      });
      if (response.status === 200) {
        const value: unknown = await response.json();
        const url = typeof value === "object" && value !== null && "authorization_url" in value && typeof value.authorization_url === "string" ? new URL(value.authorization_url) : null;
        if (url?.origin === "https://accounts.google.com" && url.pathname === "/o/oauth2/v2/auth" && url.search) {
          window.location.assign(url.toString());
          return;
        }
      }
      if (response.status === 401) setError("Check your email and current password, then try again.");
      else toast.error(response.status === 429 ? "Try again later" : "Google linking unavailable", { description: "Nothing was changed." });
    } catch {
      toast.error("Google linking unavailable", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  if (status === "loading") return <p className="text-sm text-muted-foreground" role="status">Checking linked sign-in methods…</p>;
  if (status === "linked") return <Alert variant="success"><AlertTitle>Google is linked</AlertTitle><AlertDescription>You can use the verified linked Google identity to sign in. Email matching alone never links another credential.</AlertDescription></Alert>;
  if (status === "error") return <Alert variant="warning"><AlertTitle>Google linking unavailable</AlertTitle><AlertDescription>Your existing sign-in method is unchanged.</AlertDescription></Alert>;

  return (
    <form onSubmit={submit} noValidate>
      <FieldGroup>
        <p className="text-sm text-muted-foreground">Re-enter your password before continuing to Google. Matching email addresses are never linked automatically.</p>
        <Field><FieldLabel htmlFor="google-link-email">Email address</FieldLabel><Input id="google-link-email" name="email" type="email" autoComplete="email" maxLength={254} disabled={pending} onChange={() => setError("")} /></Field>
        <Field><FieldLabel htmlFor="google-link-password">Current password</FieldLabel><PasswordInput id="google-link-password" name="password" autoComplete="current-password" maxLength={128} disabled={pending} onChange={() => setError("")} /></Field>
        {error && <FieldError>{error}</FieldError>}
        <Button type="submit" variant="outline" className="w-full sm:w-fit" disabled={pending}><GoogleIcon />{pending ? "Connecting…" : "Link Google"}</Button>
      </FieldGroup>
    </form>
  );
}
