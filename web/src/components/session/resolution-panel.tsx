"use client";

import Link from "next/link";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";
import { AccountTypeSelection } from "@/components/session/account-type-selection";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";
import type { RequiredAction } from "@/contracts/auth.generated";

const actionCopy: Record<RequiredAction, { title: string; message: string }> = {
  SELECT_ACCOUNT_TYPE: {
    title: "Finish account setup",
    message: "Choose how you’ll use Bidora. Your choice is permanent and does not grant organization access.",
  },
  ESTABLISH_TENDERPRENEUR_MEMBERSHIP: {
    title: "Choose how to join",
    message: "Accept an invitation from an existing Tenderpreneur organization, or request a new organization of your own.",
  },
  ENROLL_PLATFORM_MFA: {
    title: "Set up verification",
    message: "Two-step verification is required for Platform Administration.",
  },
  COMPLETE_PLATFORM_STEP_UP: {
    title: "Additional verification required",
    message: "Confirm with your authenticator or a recovery code to open Platform Administration. An email sign-in code alone does not provide administrator assurance.",
  },
};

export function ResolutionPanel() {
  const { resolution, refresh } = useSession();
  const { isSigningOut, signOut } = useSignOut();

  if (resolution.portal?.path === "/procurement") {
    return (
      <AuthenticationExperience title="Procurement unavailable" description="This workspace isn’t available yet.">
        <Button type="button" variant="outline" onClick={refresh}>Check again</Button>
      </AuthenticationExperience>
    );
  }

  const action = resolution.required_actions[0];
  const copy = action ? actionCopy[action] : {
    title: "No workspace available",
    message: "We couldn’t find a workspace this account can open.",
  };

  return (
    <AuthenticationExperience title={copy.title} description={copy.message}>
      {action === "SELECT_ACCOUNT_TYPE" ? <AccountTypeSelection /> : null}
      {action === "ESTABLISH_TENDERPRENEUR_MEMBERSHIP" ? (
        <Button nativeButton={false} render={<Link href="/organization/create" />}>Create an organization</Button>
      ) : null}
      {(action === "ENROLL_PLATFORM_MFA" || action === "COMPLETE_PLATFORM_STEP_UP") ? (
        <Button nativeButton={false} render={<Link href="/account/security/authenticator" />}>
          {action === "ENROLL_PLATFORM_MFA" ? "Set up authenticator" : "Verify with authenticator"}
        </Button>
      ) : null}
      {action === "COMPLETE_PLATFORM_STEP_UP" ? (
        <Button type="button" variant="outline" disabled={isSigningOut} onClick={() => void signOut()}>
          {isSigningOut ? "Signing out…" : "Sign out"}
        </Button>
      ) : null}
      {action !== "SELECT_ACCOUNT_TYPE" && action !== "COMPLETE_PLATFORM_STEP_UP" ? (
        <Button type="button" variant="outline" onClick={refresh}>Check again</Button>
      ) : null}
    </AuthenticationExperience>
  );
}
