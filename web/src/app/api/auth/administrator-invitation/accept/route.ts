import { proxyAdministratorInvitation } from "@/lib/server/administrator-invitation-proxy";
export async function POST(request:Request){return proxyAdministratorInvitation(request,"accept")}
