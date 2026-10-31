import { proxyAccountTypeSelection } from "@/lib/server/account-type-proxy";

export async function POST(request: Request) {
  return proxyAccountTypeSelection(request);
}
