"use client";

import { useCallback, useRef, useState } from "react";
import { toast } from "sonner";
import { useSession } from "@/components/session/session-provider";
import { announceSessionChange } from "@/lib/session-events";
import { submitLogout } from "@/lib/auth-client";
import { networkSubmissionError, submissionErrorMessage } from "@/lib/submission-errors";

export function useSignOut() {
  const { csrfToken } = useSession();
  const [isSigningOut, setIsSigningOut] = useState(false);
  const submitting = useRef(false);

  const signOut = useCallback(async () => {
    if (submitting.current) return;
    submitting.current = true; setIsSigningOut(true);
    toast.loading("Signing out…", { id: "sign-out" });

    try {
      const response = await submitLogout(csrfToken);
      if (response.status === 204 || response.status === 401) {
        announceSessionChange();
        toast.dismiss("sign-out");
        window.location.replace("/login");
        return;
      }

      toast.error(submissionErrorMessage(response.status, response.headers.get("retry-after")), { id: "sign-out" });
    } catch {
      toast.error(networkSubmissionError, { id: "sign-out" });
    } finally {
      submitting.current = false; setIsSigningOut(false);
    }
  }, [csrfToken]);

  return { isSigningOut, signOut };
}
