"use client";

import { useState, type FormEvent } from "react";
import { toast } from "sonner";
import { PasswordInput } from "@/components/auth/password-input";
import { usePasswordPolicy } from "@/components/auth/use-password-policy";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { submitPasswordChange } from "@/lib/auth-client";
import { isAcceptedEmail, validatePassword } from "@/lib/auth-validation";

export function PasswordChangeForm() {
 const passwordPolicy = usePasswordPolicy();
  const { csrfToken, refresh } = useSession();
  const [pending, setPending] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || passwordPolicy.minimum === null) return;
    setFieldError(null);

    const form = event.currentTarget;
    const data = new FormData(form);
    const email = String(data.get("email") ?? "");
    const currentPassword = String(data.get("current_password") ?? "");
    const newPassword = String(data.get("new_password") ?? "");
    const confirmPassword = String(data.get("confirm_password") ?? "");

    if (!isAcceptedEmail(email)) {
      setFieldError("Enter the email address for this account.");
      return;
    }
    const currentError = validatePassword(currentPassword, 1);
    if (currentError) {
      setFieldError(currentError);
      return;
    }
    const newError = validatePassword(newPassword, passwordPolicy.minimum!);
    if (newError) {
      setFieldError(newError);
      return;
    }
    if (newPassword !== confirmPassword) {
      setFieldError("New passwords do not match. Enter the same password again.");
      return;
    }

    setPending(true);
    try {
      const response = await submitPasswordChange(
        { email, current_password: currentPassword, new_password: newPassword },
        csrfToken,
      );

      if (response.status === 204) {
        form.reset();
        toast.success("Password changed", {
          description: "Your other sessions were signed out.",
        });
        refresh();
        return;
      }
      if (response.status === 400) {
        passwordPolicy.retry();
        setFieldError("Choose a new password that meets the requirements.");
        return;
      }
      if (response.status === 401) {
        setFieldError("Check your email and current password, then try again.");
        return;
      }
      if (response.status === 429) {
        toast.error("Try again later", {
          description: "Password changes are temporarily limited.",
        });
        return;
      }
      toast.error("Couldn’t change password", {
        description: response.status === 403
          ? "Refresh the page and try again."
          : "Nothing was changed. Try again.",
      });
    } catch {
      toast.error("Couldn’t change password", {
        description: "Nothing was changed. Try again.",
      });
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate>
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="password-change-email">Email address</FieldLabel>
          <Input
            id="password-change-email"
            name="email"
            type="email"
            autoComplete="email"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={254}
            placeholder="name@company.com"
            disabled={pending}
            onChange={() => setFieldError(null)}
          />
        </Field>

        <Field>
          <FieldLabel htmlFor="current-password">Current password</FieldLabel>
          <PasswordInput
            id="current-password"
            name="current_password"
            autoComplete="current-password"
            maxLength={128}
            placeholder="Enter your current password"
            disabled={pending}
            onChange={() => setFieldError(null)}
          />
        </Field>

        <Field>
          <FieldLabel htmlFor="new-password">New password</FieldLabel>
          <PasswordInput
            id="new-password"
            name="new_password"
            autoComplete="new-password"
            maxLength={128}
            placeholder="Create a new password"
            disabled={pending}
            onChange={() => setFieldError(null)}
          />
          <FieldDescription>{passwordPolicy.minimum === null ? passwordPolicy.failed ? "Password requirements are unavailable." : "Loading password requirements…" : `Use ${passwordPolicy.minimum}–128 UTF-8 bytes. Spaces are kept exactly as entered.`}</FieldDescription>
          {passwordPolicy.failed && <Button type="button" variant="outline" size="sm" onClick={passwordPolicy.retry}>Retry password requirements</Button>}
        </Field>

        <Field>
          <FieldLabel htmlFor="confirm-new-password">Confirm new password</FieldLabel>
          <PasswordInput
            id="confirm-new-password"
            name="confirm_password"
            autoComplete="new-password"
            maxLength={128}
            placeholder="Enter the same new password again"
            disabled={pending}
            onChange={() => setFieldError(null)}
          />
        </Field>

        {fieldError && <FieldError>{fieldError}</FieldError>}

        <Button type="submit" size="lg" className="w-full sm:w-fit" disabled={pending || passwordPolicy.minimum === null}>
          {pending ? "Changing password…" : "Change password"}
        </Button>
      </FieldGroup>
    </form>
  );
}
