import { proxyPlatformIntegration } from "@/lib/server/platform-proxy";
type Context={params:Promise<{operation:string}>};
export async function GET(request:Request,context:Context){return (await context.params).operation==="models"?proxyPlatformIntegration(request,"models","OPENAI"):new Response(null,{status:405,headers:{"Cache-Control":"private, no-store"}});}
export async function POST(request:Request,context:Context){const operation=(await context.params).operation;if(operation!=="test"&&operation!=="propose"&&operation!=="approve"&&operation!=="reject")return new Response(null,{status:405,headers:{"Cache-Control":"private, no-store"}});return proxyPlatformIntegration(request,operation,"OPENAI");}
