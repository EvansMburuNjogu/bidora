import assert from "node:assert/strict";
import {registerHooks} from "node:module";
import test from "node:test";

const hooks=registerHooks({resolve(specifier,context,next){
 if(specifier.startsWith("@/"))return {url:new URL(`../../src/${specifier.slice(2)}.ts`,import.meta.url).href,shortCircuit:true};
 return next(specifier,context);
}});
const {invitationNext,withInvitation}=await import("../../src/lib/invitation-code.ts");
const {parseMePortal}=await import("../../src/lib/session-contract.ts");
hooks.deregister();

test("invitation redirects carry only a valid locator on fixed local routes",()=>{
 const id="00000000-0000-4000-8000-000000000001";
 assert.equal(withInvitation("/invitations/tenant?complete=1",id),"/invitations/tenant?complete=1&invitation="+id);
 assert.equal(invitationNext("/login?invitation="+id),"/login?invitation="+id);
 for(const value of ["//outside.invalid/login","https://outside.invalid/login","javascript:alert(1)","/platform?invitation="+id,"/invitations/platform?token=secret","/login?invitation="+id+"&role=PLATFORM_OWNER"])assert.equal(invitationNext(value),null);
});

test("primary-only rotated administrator sessions can request reauthentication but never expose platform access",()=>{
 const response={data:{account_type:"PLATFORM_ADMIN",portal:null,platform_context_generation:null,capabilities:[],session:{assurance_level:"AAL1",idle_expires_at:"2030-01-01T12:00:00Z",absolute_expires_at:"2030-01-02T12:00:00Z"},required_actions:["COMPLETE_PLATFORM_STEP_UP"]},meta:{trace_id:"fixture"}};
 assert.ok(parseMePortal(response));
 assert.equal(parseMePortal({...response,data:{...response.data,portal:{key:"PLATFORM_ADMIN",path:"/platform"}}}),null);
 assert.equal(parseMePortal({...response,data:{...response.data,required_actions:[]}}),null);
 assert.equal(parseMePortal({...response,data:{...response.data,session:{...response.data.session,assurance_level:"AAL2"}}}),null);
});
