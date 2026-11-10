// Ephemeral, same-origin hints only. Never broadcast tokens, identity, roles,
// expiry values or verification evidence; receivers revalidate with the server.
export const sessionEventChannel = "bidora-session-events";
let source: string | undefined;
export function sessionEventSource() { return source ??= crypto.randomUUID(); }
export function announceSessionChange() {
 if (typeof BroadcastChannel === "undefined") return;
 const channel = new BroadcastChannel(sessionEventChannel);
 channel.postMessage({ type: "session-changed", source: sessionEventSource() });
 channel.close();
}
