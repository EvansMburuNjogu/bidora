"use client";

import { useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import type { MFAActivationResponse, MFAEnrollmentResponse, MFARecoveryCodesResponse, MFAStepUpMethod } from "@/contracts/auth.generated";
import { beginMFAReplacement, confirmMFAReplacement, regenerateMFARecoveryCodes, stepUpMFA } from "@/lib/auth-client";
import { parseMFAActivation, parseMFAEnrollment, parseMFARecoveryCodes } from "@/lib/mfa-contract";

type View =
  | { kind: "idle" }
  | { kind: "replacement"; response: MFAEnrollmentResponse }
  | { kind: "codes"; codes: string[]; title: string };

export function MFALifecycle({ onRefresh }: { onRefresh: () => void }) {
  const { csrfToken, resolution, refresh } = useSession();
  const [view, setView] = useState<View>({ kind: "idle" });
  const [method, setMethod] = useState<MFAStepUpMethod>("TOTP");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const aal2 = resolution.session.assurance_level === "AAL2";

  function normalizedCode(form: HTMLFormElement): string {
    return String(new FormData(form).get("code") ?? "").trim().toUpperCase();
  }
  function validCode(code: string, selected: MFAStepUpMethod): boolean {
    return selected === "TOTP" ? /^\d{6}$/.test(code) : /^[0-9A-HJKMNP-TV-Z]{16}$/.test(code);
  }
  function showCodes(response: MFAActivationResponse | MFARecoveryCodesResponse, title: string) {
    setView({ kind: "codes", codes: response.data.recovery_codes, title });
  }

  async function stepUp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const code = normalizedCode(event.currentTarget);
    if (!validCode(code, method)) {
      setError(method === "TOTP" ? "Enter a current six-digit code." : "Enter one complete recovery code.");
      return;
    }
    setPending(true);
    setError("");
    try {
      const response = await stepUpMFA({ code, method }, csrfToken);
      if (response.status === 204) {
        toast.success("Identity confirmed", { description: "Your session now has recent two-step verification." });
        refresh();
        return;
      }
      if (response.status === 400 || response.status === 401) setError("That code wasn’t accepted. Enter a current code and try again.");
      else toast.error(response.status === 429 ? "Try again later" : "Unable to confirm identity");
    } catch {
      toast.error("Unable to confirm identity", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  async function regenerate() {
    setPending(true);
    try {
      const response = await regenerateMFARecoveryCodes(csrfToken);
      if (response.status === 200) {
        const parsed = parseMFARecoveryCodes(await response.json());
        if (parsed) {
          showCodes(parsed, "Recovery codes replaced");
          return;
        }
      }
      toast.error(response.status === 403 ? "Recent verification required" : "Unable to replace recovery codes");
    } catch {
      toast.error("Unable to replace recovery codes", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  async function beginReplacement() {
    setPending(true);
    try {
      const response = await beginMFAReplacement(csrfToken);
      if (response.status === 201) {
        const parsed = parseMFAEnrollment(await response.json());
        if (parsed) {
          setView({ kind: "replacement", response: parsed });
          return;
        }
      }
      toast.error(response.status === 403 ? "Recent verification required" : "Unable to start authenticator replacement");
    } catch {
      toast.error("Unable to start authenticator replacement", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  async function confirmReplacement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const code = normalizedCode(form);
    if (!/^\d{6}$/.test(code)) {
      setError("Enter the six-digit code from the new authenticator.");
      return;
    }
    setPending(true);
    setError("");
    try {
      const response = await confirmMFAReplacement({ code }, csrfToken);
      if (response.status === 200) {
        const parsed = parseMFAActivation(await response.json());
        if (parsed) {
          form.reset();
          showCodes(parsed, "Authenticator replaced");
          return;
        }
      }
      if (response.status === 400 || response.status === 401) setError("That code wasn’t accepted. Enter a current code from the new authenticator.");
      else toast.error("Unable to replace authenticator", { description: "Nothing was changed. Please try again." });
    } catch {
      toast.error("Unable to replace authenticator", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  if (view.kind === "codes") {
    return (
      <div className="flex flex-col gap-4">
        <Alert variant="success" role="status"><AlertTitle>{view.title}</AlertTitle><AlertDescription>Save these codes now. Each works once and they will not be shown again.</AlertDescription></Alert>
        <ol className="grid gap-2 sm:grid-cols-2" aria-label="New recovery codes">
          {view.codes.map((code) => <li key={code} className="break-all border bg-muted/40 px-3 py-2 font-mono text-sm">{code}</li>)}
        </ol>
        <Button type="button" className="w-fit" onClick={() => { setView({ kind: "idle" }); onRefresh(); refresh(); }}>Codes saved</Button>
      </div>
    );
  }

  if (view.kind === "replacement") {
    return (
      <div className="flex flex-col gap-4">
        <Alert variant="warning" role="status"><AlertTitle>Replace your authenticator</AlertTitle><AlertDescription>Your current factor remains active until the new code is confirmed.</AlertDescription></Alert>
        <code className="break-all border bg-muted/40 px-3 py-2 font-mono text-sm">{view.response.data.secret}</code>
        <Button nativeButton={false} render={<a href={view.response.data.otpauth_uri} />} variant="outline" className="w-fit">Open authenticator</Button>
        <form onSubmit={confirmReplacement} noValidate>
          <FieldGroup><Field><FieldLabel htmlFor="mfa-replacement-code">New authenticator code</FieldLabel><Input id="mfa-replacement-code" name="code" inputMode="numeric" autoComplete="one-time-code" minLength={6} maxLength={6} disabled={pending} onChange={() => setError("")} /></Field>{error && <FieldError>{error}</FieldError>}<div className="flex flex-wrap gap-2"><Button type="submit" disabled={pending}>{pending ? "Replacing…" : "Confirm replacement"}</Button><Button type="button" variant="outline" disabled={pending} onClick={() => setView({ kind: "idle" })}>Cancel</Button></div></FieldGroup>
        </form>
      </div>
    );
  }

  if (!aal2) {
    return (
      <div className="flex flex-col gap-4">
        <Alert variant="info"><AlertTitle>Recent verification required</AlertTitle><AlertDescription>Confirm with your authenticator or one recovery code before changing MFA settings.</AlertDescription></Alert>
        <form onSubmit={stepUp} noValidate>
          <FieldGroup><Field><div className="flex flex-wrap items-center justify-between gap-2"><FieldLabel htmlFor="mfa-step-up-code">{method === "TOTP" ? "Authenticator code" : "Recovery code"}</FieldLabel><button type="button" className="text-sm font-medium text-primary underline-offset-4 hover:underline" onClick={() => { setMethod((value) => value === "TOTP" ? "RECOVERY_CODE" : "TOTP"); setError(""); }}>{method === "TOTP" ? "Use a recovery code" : "Use authenticator"}</button></div><Input id="mfa-step-up-code" name="code" inputMode={method === "TOTP" ? "numeric" : "text"} autoComplete="one-time-code" minLength={method === "TOTP" ? 6 : 16} maxLength={method === "TOTP" ? 6 : 16} disabled={pending} onChange={() => setError("")} /></Field>{error && <FieldError>{error}</FieldError>}<Button type="submit" className="w-fit" disabled={pending}>{pending ? "Confirming…" : "Confirm identity"}</Button></FieldGroup>
        </form>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Alert variant="success" role="status"><AlertTitle>Two-step verification is on</AlertTitle><AlertDescription>Your session has recent two-step verification. Sensitive changes remain server-authorized.</AlertDescription></Alert>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={pending} onClick={() => void regenerate()}>{pending ? "Working…" : "Replace recovery codes"}</Button><Button type="button" variant="outline" disabled={pending} onClick={() => void beginReplacement()}>Replace authenticator</Button></div>
    </div>
  );
}
