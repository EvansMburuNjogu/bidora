import assert from "node:assert/strict";
import test from "node:test";
import {parseOpenAISetting} from "../src/lib/platform-openai-contract.ts";
import {parsePesaPalSetting} from "../src/lib/platform-pesapal-contract.ts";
const id="11111111-1111-4111-8111-111111111111",change="22222222-2222-4222-8222-222222222222",at="2026-10-18T12:00:00Z";
const common={id,sequence:1,state:"PENDING_APPROVAL",version:3,test_status:"PASSED",test_code:"TEST_PASSED",created_at:at,tested_at:at,activated_at:null,superseded_at:null};
const ai={...common,setting_kind:"OPENAI",configuration:{setting_kind:"OPENAI",enabled:false,endpoint:"https://api.openai.com/v1/responses",model:"gpt-4.1-mini",api_key_reference_configured:true,trust_roots_reference_configured:true,store:false,timeout_seconds:60,max_concurrency:2}};
const payment={...common,setting_kind:"PESAPAL",configuration:{setting_kind:"PESAPAL",enabled:false,environment:"SANDBOX",merchant_name:"Example",credential_reference_configured:false,notification_id:null,api_endpoint:"https://cybqa.pesapal.com/pesapalv3/api"}};
const envelope=data=>({data,meta:{trace_id:id,next_cursor:null}});
for(const [kind,parse,row] of [["OpenAI",parseOpenAISetting,ai],["PesaPal",parsePesaPalSetting,payment]]){
 test(kind+" resolves optional pending request metadata without accepting secrets or arbitrary references",()=>{
  assert.ok(parse(envelope(row)));
  assert.equal(parse(envelope({...row,pending_change_id:null})).pending_change_id,null);
  assert.equal(parse(envelope({...row,pending_change_id:change})).pending_change_id,change);
  assert.equal(parse(envelope({...row,pending_change_id:"not-an-id"})),null);
  assert.equal(parse(envelope({...row,pending_change_id:change,secret:"must-not-leak"})),null);
 });
}
