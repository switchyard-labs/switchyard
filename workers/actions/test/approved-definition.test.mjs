import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedDefinition,saveApprovedDefinition} from '../src/approved-definition.mjs';
class Bucket {data=new Map();async put(k,v,o){if(o?.onlyIf&&this.data.has(k))return null;this.data.set(k,v);return {};}async get(k){const v=this.data.get(k);return v?{json:async()=>JSON.parse(v)}:null;}}
test('a new project approval preserves an admitted owner build revision',async()=>{
 const b=new Bucket();await saveApprovedDefinition(b,'repo',{revision:'owner',upload_origin:'https://worker.example',jobs:['owner']});await saveApprovedDefinition(b,'repo',{revision:'project',upload_origin:'https://worker.example',jobs:['owner','project']});
 assert.deepEqual((await approvedDefinition(b,'repo','owner')).jobs,['owner']);
 assert.deepEqual((await approvedDefinition(b,'repo','project')).jobs,['owner','project']);
 await assert.rejects(approvedDefinition(b,'repo','unknown'));
 await saveApprovedDefinition(b,'repo',{revision:'owner',upload_origin:'https://worker.example',jobs:['changed']});
 assert.deepEqual((await approvedDefinition(b,'repo','owner')).jobs,['owner']);
});
test('legacy current approvals work only for the matching revision',async()=>{const b=new Bucket();await b.put('definitions/repo.json',JSON.stringify({revision:'legacy',upload_origin:'https://worker.example'}));assert.equal((await approvedDefinition(b,'repo','legacy')).revision,'legacy');await assert.rejects(approvedDefinition(b,'repo','other'));});

test('manual and PR source refs and jobs must match the approved revision',async()=>{
 const {validateApprovedRun}=await import('../src/approved-definition.mjs');const b=new Bucket();
 const definition={revision:'revision',upload_origin:'https://worker.example',refs:['refs/heads/main'],jobs:[{id:'test',steps:[{id:'test',command:'true'}]}]};await saveApprovedDefinition(b,'repo',definition);
 const run={repo:'repo',ref:'refs/heads/main',definition_revision:'revision',jobs:definition.jobs};
 await validateApprovedRun(b,run);
 await assert.rejects(validateApprovedRun(b,{...run,ref:'refs/heads/unapproved',trigger:'pull_request'}),/ref_not_approved/);
 await assert.rejects(validateApprovedRun(b,{...run,jobs:[{id:'test',steps:[{id:'test',command:'different'}]}]}),/input_conflict/);
});
