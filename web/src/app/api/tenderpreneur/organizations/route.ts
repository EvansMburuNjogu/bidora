import { proxyOrganizationCreation } from "@/lib/server/organization-creation-proxy";

export async function GET(request: Request) {
  return proxyOrganizationCreation(request);
}

export async function POST(request: Request) {
  return proxyOrganizationCreation(request);
}
