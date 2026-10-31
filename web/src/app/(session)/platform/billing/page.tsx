import { PlatformCommercialQueues } from "@/components/platform/platform-commercial-queues";
import { commercialQueueConfig, commercialQueues, type CommercialQueue } from "@/lib/platform-commercial-contract";

export const instant = false;
export default async function BillingPage({ searchParams }: { searchParams: Promise<{ queue?: string; state?: string; reconciliation_state?: string }> }) {
  const { queue: requestedQueue, state: requestedState, reconciliation_state: requestedReconciliation } = await searchParams;
  const queue = typeof requestedQueue === "string" && commercialQueues.includes(requestedQueue as CommercialQueue) ? requestedQueue as CommercialQueue : undefined;
  const state = queue && typeof requestedState === "string" && commercialQueueConfig[queue].states.some((value) => value === requestedState) ? requestedState : undefined;
  const reconciliation = queue === "payments" && typeof requestedReconciliation === "string" && commercialQueueConfig.payments.secondary.some((value) => value === requestedReconciliation) ? requestedReconciliation : undefined;
  return <PlatformCommercialQueues key={`${queue ?? "default"}-${state ?? ""}-${reconciliation ?? ""}`} initialQueue={queue} initialState={state} initialReconciliation={reconciliation} />;
}
