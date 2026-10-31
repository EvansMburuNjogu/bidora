"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MePortalResponse } from "@/contracts/auth.generated";
import { parseMePortal, parseSessionBootstrap } from "@/lib/session-contract";
import { SessionStatus } from "@/components/session/session-status";

type SessionValue = {
  csrfToken: string;
  resolution: MePortalResponse["data"];
  refresh: () => void;
};

type State =
  | { status: "loading" }
  | { status: "unauthenticated" }
  | { status: "error" }
  | { status: "ready"; csrfToken: string; response: MePortalResponse };

const SessionContext = createContext<SessionValue | null>(null);
const supportedPortalPaths = new Set(["/tenderpreneur", "/platform"]);
const sharedAccountPaths = new Set(["/account/security", "/account/security/authenticator", "/account/security/google-linked", "/account/security/google-link-failed", "/organization/create"]);

function within(pathname: string, root: string): boolean {
  return pathname === root || pathname.startsWith(`${root}/`);
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [generation, setGeneration] = useState(0);
  const [state, setState] = useState<State>({ status: "loading" });

  const refresh = useCallback(() => {
    setState({ status: "loading" });
    setGeneration((value) => value + 1);
  }, []);

  useEffect(() => {
    const controller = new AbortController();

    async function load() {
      try {
        const bootstrapResponse = await fetch("/api/session/bootstrap", {
          credentials: "include",
          cache: "no-store",
          headers: { Accept: "application/json" },
          signal: controller.signal,
        });
        if (bootstrapResponse.status === 401) {
          setState({ status: "unauthenticated" });
          return;
        }
        if (!bootstrapResponse.ok) {
          setState({ status: "error" });
          return;
        }
        const bootstrap = parseSessionBootstrap(await bootstrapResponse.json());
        if (!bootstrap) {
          setState({ status: "error" });
          return;
        }

        const meResponse = await fetch("/api/session/me", {
          credentials: "include",
          cache: "no-store",
          headers: { Accept: "application/json" },
          signal: controller.signal,
        });
        if (meResponse.status === 401) {
          setState({ status: "unauthenticated" });
          return;
        }
        if (!meResponse.ok) {
          setState({ status: "error" });
          return;
        }
        const me = parseMePortal(await meResponse.json());
        if (!me) {
          setState({ status: "error" });
          return;
        }
        setState({ status: "ready", csrfToken: bootstrap.data.csrf_token, response: me });
      } catch (error) {
        if (!(error instanceof DOMException && error.name === "AbortError")) setState({ status: "error" });
      }
    }

    void load();
    return () => controller.abort();
  }, [generation]);

  useEffect(() => {
    if (state.status !== "ready") return;
    const idleExpiry = Date.parse(state.response.data.session.idle_expires_at);
    const absoluteExpiry = Date.parse(state.response.data.session.absolute_expires_at);
    const delay = Math.max(0, Math.min(idleExpiry, absoluteExpiry) - Date.now() + 250);
    const timeout = window.setTimeout(refresh, Math.min(delay, 2_147_483_647));
    return () => window.clearTimeout(timeout);
  }, [refresh, state]);

  useEffect(() => {
    function revalidateVisibleSession() {
      if (document.visibilityState === "visible") refresh();
    }
    document.addEventListener("visibilitychange", revalidateVisibleSession);
    return () => document.removeEventListener("visibilitychange", revalidateVisibleSession);
  }, [refresh]);

  useEffect(() => {
    if (state.status === "unauthenticated") {
      router.replace("/login");
      return;
    }
    if (state.status !== "ready") return;

    const destination = state.response.data.portal?.path ?? null;
    const onPortalRoute = within(pathname, "/tenderpreneur") || within(pathname, "/platform");
    const onDestinationRoute = destination !== null && within(pathname, destination);
    const onSharedAccountRoute = sharedAccountPaths.has(pathname);

    if (!destination && onPortalRoute) {
      router.replace("/auth/resolve");
    } else if (destination && supportedPortalPaths.has(destination) && !onDestinationRoute && !onSharedAccountRoute) {
      router.replace(destination);
    } else if (destination === "/procurement" && onPortalRoute) {
      router.replace("/auth/resolve");
    }
  }, [pathname, router, state]);

  const value = useMemo<SessionValue | null>(
    () =>
      state.status === "ready"
        ? { csrfToken: state.csrfToken, resolution: state.response.data, refresh }
        : null,
    [refresh, state],
  );

  if (state.status === "loading" || state.status === "unauthenticated") {
    return <SessionStatus kind="loading" onRetry={refresh} />;
  }
  if (state.status === "error") {
    return <SessionStatus kind="error" onRetry={refresh} />;
  }

  const destination = state.response.data.portal?.path ?? null;
  const onPortalRoute = within(pathname, "/tenderpreneur") || within(pathname, "/platform");
  const onDestinationRoute = destination !== null && within(pathname, destination);
  const onSharedAccountRoute = sharedAccountPaths.has(pathname);
  const redirecting =
    (destination !== null && supportedPortalPaths.has(destination) && !onDestinationRoute && !onSharedAccountRoute) ||
    (!destination && onPortalRoute) ||
    (destination === "/procurement" && onPortalRoute);

  if (redirecting) return <SessionStatus kind="loading" onRetry={refresh} />;

  return <SessionContext value={value}>{children}</SessionContext>;
}

export function useSession(): SessionValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error("session_context_missing");
  return value;
}
