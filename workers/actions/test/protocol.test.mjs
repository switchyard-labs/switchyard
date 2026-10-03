import {test} from 'node:test';import assert from 'node:assert/strict';
import {validateRun,signature,equalSignature} from '../src/protocol.mjs';
const run=()=>({provider:'cloudflare-artifacts',providerData:{namespace:'test'},owner:'test',repo:'repo',sha:'a'.repeat(40),ref:'refs/heads/main',run_id:'run-1',definition_revision:'b'.repeat(64),jobs:[{id:'test',steps:[{id:'unit',command:'node --test',timeout_ms:1000}]}]});
test('bind repository, namespace, immutable SHA and definition',()=>{assert.equal(validateRun(run(),'test',['repo']).sha,'a'.repeat(40));for(const patch of [{sha:'main'},{repo:'private'},{owner:'other'},{definition_revision:'v1'},{ref:'refs/heads/a/../main'}])assert.throws(()=>validateRun({...run(),...patch},'test',['repo']));});
test('reject excessive or malformed commands and duplicate steps',()=>{let value=run();value.jobs[0].steps[0].timeout_ms=120001;assert.throws(()=>validateRun(value,'test',['repo']));value=run();value.jobs[0].steps.push({...value.jobs[0].steps[0]});assert.throws(()=>validateRun(value,'test',['repo']));});
test('HMAC binds body, method, query and timestamp',async()=>{const first=await signature('secret','1234567890','POST','/dispatch','body');assert.ok(equalSignature(first,first));for(const args of [['secret','1234567891','POST','/dispatch','body'],['secret','1234567890','GET','/dispatch','body'],['secret','1234567890','POST','/dispatch?x=1','body'],['secret','1234567890','POST','/dispatch','other']])assert.ok(!equalSignature(first,await signature(...args)));assert.ok(!equalSignature('',first));});

test('request body stops reading at the byte limit', async()=>{
 const {boundedBody,maxRequestBytes}=await import('../src/protocol.mjs');let cancelled=false;
 const stream=new ReadableStream({start(controller){controller.enqueue(new Uint8Array(maxRequestBytes+1));},cancel(){cancelled=true;}});
 await assert.rejects(()=>boundedBody({body:stream}),/request_too_large/);assert.equal(cancelled,true);
 assert.equal(await boundedBody(new Request('https://example.com',{method:'POST',body:'héllo'})),'héllo');
});

test('reject colliding job-step log identities',()=>{
 const value=run();value.jobs=[{id:'a-b',steps:[{id:'c',command:'true',timeout_ms:1000}]},{id:'a',steps:[{id:'b-c',command:'true',timeout_ms:1000}]}];assert.throws(()=>validateRun(value,'test',['repo']),/ambiguous_step_identity/);
});

test('failed-job rerun rejects a different commit and successful-job substitution',async()=>{
 const {validateRerunParent}=await import('../src/protocol.mjs');
 const jobs=[{id:'passed',steps:[{id:'test',command:'true',timeout_ms:1000}]},{id:'failed',steps:[{id:'test',command:'false',timeout_ms:1000}]}];
 const run={repo:'fixture',sha:'a'.repeat(40),definition_revision:'b'.repeat(64),jobs,selected_jobs:['failed']};
 const previous={run:{...run},status:'failed',jobs:[{id:'passed',status:'succeeded'},{id:'failed',status:'failed'}]};
 assert.equal(validateRerunParent(run,previous),previous);
 assert.throws(()=>validateRerunParent({...run,sha:'c'.repeat(40)},previous));
 assert.throws(()=>validateRerunParent({...run,selected_jobs:['passed']},previous));
 assert.throws(()=>validateRerunParent({...run,jobs:[{...jobs[0],steps:[{...jobs[0].steps[0],command:'different'}]},jobs[1]]},previous));
});
