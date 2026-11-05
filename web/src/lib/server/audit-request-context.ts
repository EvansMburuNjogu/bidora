import "server-only";
import { clientNetwork } from "@/lib/server/client-network";

// Use only the ingress-overwritten client address. Never forward a caller's
// X-Forwarded-For chain. User-Agent is bounded, untrusted display metadata.
export function forwardAuditContext(request: Request, headers: Headers): void {
  const network = clientNetwork(request);
  if (network.ok && network.address) headers.set("X-Forwarded-For", network.address);
  const agent = request.headers.get("user-agent");
  if (agent && agent.length <= 512 && !/[\u0000-\u001f\u007f]/.test(agent)) headers.set("User-Agent", agent);
}
