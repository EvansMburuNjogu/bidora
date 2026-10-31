import { proxyIntegrationRead } from "@/lib/server/platform-smtp-read-proxy";
export async function GET(request:Request,context:{params:Promise<{changeId:string}>}){return proxyIntegrationRead(request,"OPENAI",(await context.params).changeId);}
