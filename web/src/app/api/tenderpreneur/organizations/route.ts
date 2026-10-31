import { proxyOrganizationCreation } from "@/lib/server/organization-creation-proxy";

export async function POST(request: Request) {
  return proxyOrganizationCreation(request);
}
