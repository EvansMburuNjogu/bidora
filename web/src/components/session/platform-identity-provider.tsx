"use client";

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { parsePlatformInformation, type PlatformInformation } from "@/lib/platform-information-contract";

type Identity = { information: PlatformInformation | null; refresh: () => void };
const IdentityContext = createContext<Identity>({ information: null, refresh: () => {} });
export function PlatformIdentityProvider({ children }: { children: ReactNode }) {
  const [information, setInformation] = useState<PlatformInformation | null>(null);
  const [revision, setRevision] = useState(0);
  const refresh = useCallback(() => setRevision((value) => value + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch("/api/session/platform-information", { credentials: "include", cache: "no-store", signal: AbortSignal.any([controller.signal, AbortSignal.timeout(10_000)]) });
        const value = response.ok ? parsePlatformInformation(await response.json()) : null;
        if (!controller.signal.aborted) setInformation(value);
      } catch { if (!controller.signal.aborted) setInformation(null); }
    })();
    return () => controller.abort();
  }, [revision]);
  return <IdentityContext value={{ information, refresh }}>{children}</IdentityContext>;
}
export function usePlatformIdentity() { return useContext(IdentityContext); }
export function PlatformName() { return <>{usePlatformIdentity().information?.platform_name ?? "Bidora"}</>; }
export function PlatformContact() {
  const { information } = usePlatformIdentity();
  if (!information) return null;
  return <span className="inline-flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
    {information.support_email && <a href={`mailto:${encodeURIComponent(information.support_email)}`} className="hover:underline">Contact support</a>}
    {information.website && <a href={information.website} target="_blank" rel="noopener noreferrer" className="hover:underline">Website</a>}
    {information.contact_phone && <a href={`tel:${information.contact_phone.replace(/[ ()-]/g, "")}`} className="hover:underline">{information.contact_phone}</a>}
  </span>;
}
