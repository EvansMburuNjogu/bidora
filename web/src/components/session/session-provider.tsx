"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { MePortalResponse } from "@/contracts/auth.generated";
import { parseMePortal, parseSessionBootstrap } from "@/lib/session-contract";
import { announceSessionChange, sessionEventChannel, sessionEventSource } from "@/lib/session-events";
import { SessionStatus } from "@/components/session/session-status";

type SessionValue = { csrfToken: string; resolution: MePortalResponse["data"]; refresh: () => void };
type Ready = { status: "ready"; csrfToken: string; response: MePortalResponse; clockOffset: number; degraded: boolean };
type State = { status: "loading" | "unauthenticated" | "error" } | Ready;
const SessionContext = createContext<SessionValue | null>(null);
const supportedPortalPaths = new Set(["/tenderpreneur", "/platform"]);
const sharedAccountPaths = new Set(["/account/security", "/account/security/authenticator", "/account/security/google-linked", "/account/security/google-link-failed", "/organization/create"]);
function within(pathname: string, root: string): boolean { return pathname === root || pathname.startsWith(root + "/"); }
function deadline(state: Ready) { return Math.min(Date.parse(state.response.data.session.idle_expires_at), Date.parse(state.response.data.session.absolute_expires_at)); }
function validCachedLease(state: State): state is Ready { return state.status === "ready" && Date.now() + state.clockOffset < deadline(state); }

export function SessionProvider({ children }: { children: ReactNode }) {
 const pathname = usePathname(), router = useRouter();
 const [generation, setGeneration] = useState(0), [state, setState] = useState<State>({ status: "loading" });
 const recheckPending = useRef(false);
 const leavingExpiredSession=useRef(false);
 const checking = useRef(false), lastRenewed = useRef(0), activity = useRef(false), initiallyAnnounced = useRef(false);
 const refresh = useCallback(() => {
  if (checking.current) { recheckPending.current = true; return; }
  checking.current = true;
  setState(current => validCachedLease(current) ? current : { status: "loading" });
  setGeneration(value => value + 1);
 }, []);

 useEffect(() => {
  const controller = new AbortController(), signal = AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]);
  checking.current = true;
  function update(next: State) { if (!controller.signal.aborted) setState(next); }
  function unavailable() {
   if (!controller.signal.aborted) setState(current => validCachedLease(current) ? { ...current, degraded: true } : { status: "error" });
  }
  async function load() {
   try {
    const bootstrapResponse = await fetch("/api/session/bootstrap", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal });
    signal.throwIfAborted();
    if (bootstrapResponse.status === 401) { if (!recheckPending.current) update({ status: "unauthenticated" }); return; }
    if (!bootstrapResponse.ok) { unavailable(); return; }
    const bootstrap = parseSessionBootstrap(await bootstrapResponse.json()); signal.throwIfAborted();
    if (!bootstrap) { unavailable(); return; }
    const response = await fetch("/api/session/me", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal });
    signal.throwIfAborted();
    if (response.status === 401) { if (!recheckPending.current) update({ status: "unauthenticated" }); return; }
    if (!response.ok) { unavailable(); return; }
    const me = parseMePortal(await response.json()); signal.throwIfAborted();
    if (!me) { unavailable(); return; }
    const observed = Date.parse(response.headers.get("date") ?? "");
    update({ status: "ready", csrfToken: bootstrap.data.csrf_token, response: me, clockOffset: Number.isFinite(observed) ? observed - Date.now() : 0, degraded: false });
    if (!initiallyAnnounced.current) { initiallyAnnounced.current = true; announceSessionChange(); }
   } catch { if (!controller.signal.aborted) unavailable(); }
   finally { if (!controller.signal.aborted) { checking.current = false; if (recheckPending.current) { recheckPending.current = false; refresh(); } } }
  }
  void load();
  return () => { controller.abort(); checking.current = false; };
 }, [generation, refresh]);

 // Passive reads never renew a lease. Only trusted interaction followed by an
 // Origin/CSRF-protected server request can extend idle time, never absolute time
 // or email/step-up assurance. No token rotation race is introduced.
 const csrf = state.status === "ready" ? state.csrfToken : null;
 useEffect(() => {
  if (!csrf) return;
  const controller = new AbortController(); let renewing = false;
  async function performRenewal() {
   if (renewing || checking.current || !activity.current || document.visibilityState !== "visible" || performance.now() - lastRenewed.current < 60_000) return;
   renewing = true;
   const renew = async () => {
    // A sibling tab may have renewed while this tab waited for the browser lock.
    if (performance.now() - lastRenewed.current < 60_000 || controller.signal.aborted) return;
    lastRenewed.current = performance.now();
    activity.current = false;
    try {
     const response = await fetch("/api/session/renew", { method: "POST", credentials: "include", cache: "no-store", headers: { Accept: "application/json", "X-CSRF-Token": csrf! }, signal: AbortSignal.any([controller.signal, AbortSignal.timeout(10_000)]) });
     if (controller.signal.aborted) return;
     if (response.status === 401) { setState({status:"unauthenticated"});return; }
     if (response.status === 403) { refresh(); return; }
     if (!response.ok) { activity.current = true; setState(current => validCachedLease(current) ? { ...current, degraded: true } : current); return; }
     const value = await response.json(), data = value?.data;
     const idle = Date.parse(data?.idle_expires_at), absolute = Date.parse(data?.absolute_expires_at), observed = Date.parse(data?.observed_at);
     if (!data || !Number.isFinite(idle) || !Number.isFinite(absolute) || !Number.isFinite(observed) || idle > absolute || idle <= observed) { activity.current = true; return; }
     if (controller.signal.aborted) return;
     setState(current => current.status === "ready" && current.csrfToken === csrf ? absolute > Date.parse(current.response.data.session.absolute_expires_at) ? { ...current, degraded: true } : { ...current, degraded: false, clockOffset: observed - Date.now(), response: { ...current.response, data: { ...current.response.data, session: { ...current.response.data.session, idle_expires_at: data.idle_expires_at, absolute_expires_at: data.absolute_expires_at } } } } : current);
     announceSessionChange();
    } catch { if (!controller.signal.aborted) { activity.current = true; setState(current => validCachedLease(current) ? { ...current, degraded: true } : current); } }
   };
   try {
    if (navigator.locks) await navigator.locks.request("bidora-session-renew", { ifAvailable: true }, async lock => { if (lock) await renew(); });
    else await renew(); // The monotonic server update also makes concurrent tabs safe.
   } catch { if (!controller.signal.aborted) await renew(); }
   finally { renewing = false; }
  }
  const interacted = (event: Event) => { if (event.isTrusted) { activity.current = true; void performRenewal(); } };
  for (const event of ["pointerdown", "keydown", "wheel", "touchstart"]) document.addEventListener(event, interacted, { passive: true });
  const timer = window.setInterval(() => void performRenewal(), 30_000);
  return () => { controller.abort(); window.clearInterval(timer); for (const event of ["pointerdown", "keydown", "wheel", "touchstart"]) document.removeEventListener(event, interacted); };
 }, [csrf, refresh]);

 useEffect(() => {
  if (state.status !== "ready") return;
  const delay = Math.max(1000, deadline(state) - (Date.now() + state.clockOffset) + 1000);
  const timeout = window.setTimeout(refresh, Math.min(delay, 2_147_483_647));
  return () => window.clearTimeout(timeout);
 }, [refresh, state]);

 useEffect(() => {
  const visible = () => { if (document.visibilityState === "visible" && !checking.current) refresh(); };
  const restored = (event: PageTransitionEvent) => { if (event.persisted) refresh(); };
  document.addEventListener("visibilitychange", visible); window.addEventListener("focus",visible); window.addEventListener("pageshow", restored);
  const channel = typeof BroadcastChannel !== "undefined" ? new BroadcastChannel(sessionEventChannel) : null;
  if (channel) channel.onmessage = event => { if (event.data?.type === "session-changed" && event.data?.source !== sessionEventSource()) { lastRenewed.current = performance.now(); refresh(); } };
  return () => { document.removeEventListener("visibilitychange", visible); window.removeEventListener("focus",visible); window.removeEventListener("pageshow", restored); channel?.close(); };
 }, [refresh]);

 useEffect(() => {
  if (state.status === "unauthenticated") {
   if(!leavingExpiredSession.current){leavingExpiredSession.current=true;announceSessionChange();window.location.replace("/login");}
   return;
  }
  if (state.status !== "ready") return;
  if(state.response.data.required_actions.includes("COMPLETE_PLATFORM_STEP_UP")&&pathname!=="/auth/resolve"){router.replace("/auth/resolve");return;}
  const destination = state.response.data.portal?.path ?? null;
  const onPortal = within(pathname, "/tenderpreneur") || within(pathname, "/platform");
  if (!destination && onPortal || destination === "/procurement" && onPortal) router.replace("/auth/resolve");
  else if (destination && supportedPortalPaths.has(destination) && !within(pathname, destination) && !sharedAccountPaths.has(pathname)) router.replace(destination);
 }, [pathname, router, state]);

 const value = useMemo<SessionValue | null>(() => state.status === "ready" ? { csrfToken: state.csrfToken, resolution: state.response.data, refresh } : null, [refresh, state]);
 if (state.status === "loading" || state.status === "unauthenticated") return <SessionStatus kind="loading" onRetry={refresh} />;
 if (state.status === "error") return <SessionStatus kind="error" onRetry={refresh} />;
 // The non-ready variants have already returned; this also keeps TS narrowing explicit.
 if (state.status !== "ready") return null;
 const destination = state.response.data.portal?.path ?? null, onPortal = within(pathname, "/tenderpreneur") || within(pathname, "/platform");
 const redirecting = state.response.data.required_actions.includes("COMPLETE_PLATFORM_STEP_UP")&&pathname!=="/auth/resolve"||destination !== null && supportedPortalPaths.has(destination) && !within(pathname, destination) && !sharedAccountPaths.has(pathname) || !destination && onPortal || destination === "/procurement" && onPortal;
 if (redirecting) return <SessionStatus kind="loading" onRetry={refresh} />;
 const contextKey = `${state.csrfToken}:${state.response.data.account_type}:${state.response.data.platform_context_generation}:${destination}`;
 return <SessionContext key={contextKey} value={value}>{state.degraded && <div role="status" className="border-b bg-muted px-4 py-2 text-sm">Connection interrupted. Your session has not been signed out. <button type="button" className="underline" onClick={refresh}>Retry</button></div>}{children}</SessionContext>;
}
export function useSession(): SessionValue {
 const value = useContext(SessionContext); if (!value) throw new Error("session_context_missing"); return value;
}
