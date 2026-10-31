"use client";

import { useEffect, useState } from "react";
import { GoogleIcon } from "@/components/auth/google-icon";
import { Button } from "@/components/ui/button";
import { FieldSeparator } from "@/components/ui/field";

export function GoogleAuthButton({ registration, disabled }: { registration: boolean; disabled: boolean }) {
  const [availability, setAvailability] = useState<{ enabled: boolean; allow_signup: boolean } | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      try { const response = await fetch("/api/auth/google/status", { cache: "no-store", signal: AbortSignal.any([controller.signal, AbortSignal.timeout(5000)]) }); const result: unknown = response.ok ? await response.json() : null; if (controller.signal.aborted) return; if (result && typeof result === "object" && "data" in result && result.data && typeof result.data === "object" && "enabled" in result.data && typeof result.data.enabled === "boolean" && "allow_signup" in result.data && typeof result.data.allow_signup === "boolean") setAvailability({ enabled: result.data.enabled, allow_signup: result.data.allow_signup }); else setAvailability(null); }
      catch { if (!controller.signal.aborted) setAvailability(null); }
    };
    void load(); const visible = () => { if (document.visibilityState === "visible") void load(); }; document.addEventListener("visibilitychange", visible);
    return () => { controller.abort(); document.removeEventListener("visibilitychange", visible); };
  }, []);
  if (!availability?.enabled || registration && !availability.allow_signup) return null;
  return <><FieldSeparator>Or continue with</FieldSeparator><form action="/api/auth/google-start" method="post"><Button type="submit" variant="outline" size="lg" className="w-full justify-center" disabled={disabled}><GoogleIcon />Continue with Google</Button></form></>;
}
