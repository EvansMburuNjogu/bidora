"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";

type State = "checking" | "error";

export function GoogleCompletion() {
  const router = useRouter();
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<State>("checking");

  const retry = useCallback(() => {
    setState("checking");
    setAttempt((value) => value + 1);
  }, []);

  useEffect(() => {
    const controller = new AbortController();

    async function resolve() {
      try {
        const response = await fetch("/api/session/bootstrap", {
          credentials: "include",
          cache: "no-store",
          headers: { Accept: "application/json" },
          signal: controller.signal,
        });
        if (response.ok) {
          router.replace("/auth/resolve");
          return;
        }
        if (response.status === 401) {
          router.replace("/auth/login-code");
          return;
        }
        setState("error");
        toast.error("Sign-in incomplete", { id: "google-completion-error", description: "Try again or use your email and password." });
      } catch (error) {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          setState("error");
          toast.error("Sign-in incomplete", { id: "google-completion-error", description: "Try again or use your email and password." });
        }
      }
    }

    void resolve();
    return () => controller.abort();
  }, [attempt, router]);

  if (state === "checking") {
    return <p className="type-body text-muted-foreground" role="status">Checking your sign-in…</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      <Button type="button" onClick={retry}>Try again</Button>
      <Button nativeButton={false} render={<Link href="/login" />} variant="outline">Use email and password</Button>
    </div>
  );
}
