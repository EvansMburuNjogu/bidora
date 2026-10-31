"use client";

import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { useSession } from "@/components/session/session-provider";
import { MFALifecycle } from "@/components/auth/mfa-lifecycle";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import type { MFAActivationResponse, MFAEnrollmentResponse, MFAReadinessResponse } from "@/contracts/auth.generated";
import { beginMFAEnrollment, confirmMFAEnrollment } from "@/lib/auth-client";
import { parseMFAActivation, parseMFAEnrollment, parseMFAReadiness } from "@/lib/mfa-contract";

type State =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; response: MFAReadinessResponse }
  | { status: "pending"; response: MFAEnrollmentResponse }
  | { status: "activated"; response: MFAActivationResponse };

export function MFAEnrollment() {
  const { csrfToken, refresh } = useSession();
  const [generation, setGeneration] = useState(0);
  const [state, setState] = useState<State>({ status: "loading" });
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    async function load() {
      try {
        const response = await fetch("/api/auth/mfa/preparation", {
          credentials: "include",
          cache: "no-store",
          headers: { Accept: "application/json" },
          signal: controller.signal,
        });
        if (!response.ok) {
          setState({ status: "error" });
          return;
        }
        const parsed = parseMFAReadiness(await response.json());
        setState(parsed ? { status: "ready", response: parsed } : { status: "error" });
      } catch (error) {
        if (!(error instanceof DOMException && error.name === "AbortError")) setState({ status: "error" });
      }
    }

    void load();
    return () => controller.abort();
  }, [generation]);

  function retry() {
    setState({ status: "loading" });
    setGeneration((value) => value + 1);
  }

  async function beginEnrollment() {
    if (pending) return;
    setPending(true);
    try {
      const response = await beginMFAEnrollment({}, csrfToken);
      if (response.status === 201) {
        const parsed = parseMFAEnrollment(await response.json());
        if (parsed) {
          setState({ status: "pending", response: parsed });
          return;
        }
      }
      if (response.status === 409) {
        retry();
        return;
      }
      toast.error("Couldn’t start verification setup", {
        description: response.status === 400
          ? "Sign out and back in before trying again."
          : "Nothing was changed. Try again.",
      });
    } catch {
      toast.error("Couldn’t start verification setup", {
        description: "Nothing was changed. Try again.",
      });
    } finally {
      setPending(false);
    }
  }

  async function confirmEnrollment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFieldError(null);
    const form = event.currentTarget;
    const code = String(new FormData(form).get("code") ?? "").trim();
    if (!/^\d{6}$/.test(code)) {
      setFieldError("Enter the six-digit code from your authenticator app.");
      return;
    }

    setPending(true);
    try {
      const response = await confirmMFAEnrollment({ code }, csrfToken);
      if (response.status === 200) {
        const parsed = parseMFAActivation(await response.json());
        if (parsed) {
          form.reset();
          setState({ status: "activated", response: parsed });
          return;
        }
      }
      if (response.status === 400 || response.status === 401) {
        setFieldError("That code wasn’t accepted. Enter a current code and try again.");
        return;
      }
      if (response.status === 429) {
        toast.error("Enrollment expired", {
          description: "Start verification setup again.",
        });
        retry();
        return;
      }
      toast.error("Couldn’t enable two-step verification", {
        description: "Nothing was changed. Try again.",
      });
    } catch {
      toast.error("Couldn’t enable two-step verification", {
        description: "Nothing was changed. Try again.",
      });
    } finally {
      setPending(false);
    }
  }

  if (state.status === "loading") {
    return <p className="type-body text-muted-foreground" role="status">Checking verification status…</p>;
  }

  if (state.status === "error") {
    return (
      <div className="flex flex-col gap-4">
        <Alert variant="destructive" role="alert">
          <AlertTitle>Verification status unavailable</AlertTitle>
          <AlertDescription>Nothing was changed. Try again.</AlertDescription>
        </Alert>
        <Button type="button" variant="outline" onClick={retry}>Try again</Button>
      </div>
    );
  }

  if (state.status === "activated") {
    return (
      <div className="flex flex-col gap-5">
        <Alert variant="success" role="status">
          <AlertTitle>Two-step verification enabled</AlertTitle>
          <AlertDescription>Save these recovery codes now. Each code works once and will not be shown again.</AlertDescription>
        </Alert>
        <ol className="grid min-w-0 gap-2 sm:grid-cols-2" aria-label="Recovery codes">
          {state.response.data.recovery_codes.map((code) => (
            <li key={code} className="min-w-0 break-all border bg-muted/40 px-3 py-2 font-mono text-sm">{code}</li>
          ))}
        </ol>
        <Button type="button" size="lg" className="w-full sm:w-fit" onClick={refresh}>Codes saved</Button>
      </div>
    );
  }

  if (state.status === "pending") {
    return (
      <div className="flex flex-col gap-5">
        <Alert variant="info" role="status">
          <AlertTitle>Add Bidora to your authenticator</AlertTitle>
          <AlertDescription>Open the setup link or enter the secret manually, then confirm with a current six-digit code.</AlertDescription>
        </Alert>
        <div className="flex flex-col gap-2">
          <span className="type-kicker text-muted-foreground">Manual setup secret</span>
          <code className="min-w-0 break-all border bg-muted/40 px-3 py-2 font-mono text-sm">{state.response.data.secret}</code>
        </div>
        <Button
          nativeButton={false}
          render={<a href={state.response.data.otpauth_uri} />}
          variant="outline"
          size="lg"
          className="w-full sm:w-fit"
        >
          Open authenticator
        </Button>
        <form onSubmit={confirmEnrollment} noValidate>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="mfa-enrollment-code">Authenticator code</FieldLabel>
              <Input
                id="mfa-enrollment-code"
                name="code"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{6}"
                minLength={6}
                maxLength={6}
                placeholder="123456"
                disabled={pending}
                onChange={() => setFieldError(null)}
              />
            </Field>
            {fieldError && <FieldError>{fieldError}</FieldError>}
            <Button type="submit" size="lg" className="w-full sm:w-fit" disabled={pending}>
              {pending ? "Enabling…" : "Enable verification"}
            </Button>
          </FieldGroup>
        </form>
      </div>
    );
  }

  if (state.response.data.mfa_enrolled) {
    return <MFALifecycle onRefresh={retry} />;
  }

  const freshSession = state.response.data.session_age_seconds <= 900;
  return (
    <div className="flex flex-col gap-4">
      {!freshSession && (
        <Alert variant="warning" role="status">
          <AlertTitle>Recent sign-in required</AlertTitle>
          <AlertDescription>Sign out and back in before setting up two-step verification.</AlertDescription>
        </Alert>
      )}
      <p className="type-body text-muted-foreground">
        Use an authenticator app to add a second verification step when you sign in.
      </p>
      <Button type="button" size="lg" className="w-full sm:w-fit" disabled={pending || !freshSession || !state.response.data.mfa_enrollment_available} onClick={() => void beginEnrollment()}>
        {pending ? "Starting setup…" : "Set up authenticator"}
      </Button>
    </div>
  );
}
