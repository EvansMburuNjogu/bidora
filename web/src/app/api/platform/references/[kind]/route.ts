import { proxyPlatformDashboard } from "@/lib/server/platform-dashboard-proxy";
export async function GET(request: Request, context: { params: Promise<{kind:string}> }) {
 const {kind}=await context.params;
 if(kind!=="organizations"&&kind!=="support-assignees")return Response.json({error:{message:"Search not found."}},{status:404,headers:{"Cache-Control":"private, no-store"}});
 return proxyPlatformDashboard(request,`references/${kind}`);
}
