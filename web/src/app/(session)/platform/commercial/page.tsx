import { PlatformCommercialQueues } from "@/components/platform/platform-commercial-queues";
import { commercialQueues, type CommercialQueue } from "@/lib/platform-commercial-contract";

export const instant = false;
export default async function CommercialPage({ searchParams }: { searchParams: Promise<{ queue?: string }> }) {
  const { queue } = await searchParams;
  const initialQueue = typeof queue === "string" && commercialQueues.includes(queue as CommercialQueue) ? queue as CommercialQueue : undefined;
  return <PlatformCommercialQueues key={initialQueue ?? "default"} initialQueue={initialQueue} />;
}
