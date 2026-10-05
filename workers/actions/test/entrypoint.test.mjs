import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {signature} from '../src/protocol.mjs';
import {saveApprovedDefinition} from '../src/approved-definition.mjs';

test('actual Worker fetch enforces HMAC and approved ref before durable dispatch',async()=>{
 const dir=await mkdtemp(join(tmpdir(),'switchyard-entrypoint-'));
 try {
 const outfile=join(dir,'worker.mjs');
 // Cloudflare runtime classes are the only mocked imports. The fetch route,
 // protocol, approved-definition policy and receipt code are the real bundle.
 await build({entryPoints:['src/index.ts'],outfile,bundle:true,format:'esm',platform:'node',plugins:[{name:'runtime',setup(b){b.onResolve({filter:/^@cloudflare\/ci(?:\/worker)?$/},args=>({path:args.path,namespace:'runtime'}));b.onLoad({filter:/.*/,namespace:'runtime'},()=>({contents:'export class CIWorkflow {} export class CiSandbox {} export const isCiRunnerFailure=()=>false;',loader:'js'}));}}]});
 const worker=(await import(pathToFileURL(outfile))).default;
 const data=new Map();const bucket={async get(key){const value=data.get(key);return value?{json:async()=>JSON.parse(value)}:null;},async put(key,value,options){if(options?.onlyIf&&data.has(key))return null;data.set(key,value);return {};}};
 let creates=0;const env={CONTROL_SECRET:'fixture-secret',ARTIFACTS_NAMESPACE:'fixture',ALLOWED_REPOS:'repo',BACKUP_BUCKET:bucket,CI_WORKFLOW:{async create({id}){creates++;return {id};}}};
 const jobs=[{id:'test',steps:[{id:'test',command:'true',timeout_ms:1000}]}];
 const run={provider:'cloudflare-artifacts',providerData:{namespace:'fixture'},owner:'fixture',repo:'repo',sha:'a'.repeat(40),ref:'refs/heads/main',run_id:'entrypoint-run',definition_revision:'b'.repeat(64),jobs};
 await saveApprovedDefinition(bucket,'repo',{revision:run.definition_revision,upload_origin:'https://worker.example',refs:['refs/heads/main'],jobs});
 async function request(value,signed=true){const body=JSON.stringify(value),timestamp=String(Math.floor(Date.now()/1000));const headers={'Content-Type':'application/json','X-Switchyard-Time':timestamp};if(signed)headers['X-Switchyard-Signature']=await signature(env.CONTROL_SECRET,timestamp,'POST','/dispatch',body);return worker.fetch(new Request('https://worker.example/dispatch',{method:'POST',headers,body}),env);}
 assert.equal((await request(run,false)).status,401);
 assert.equal((await request({...run,ref:'refs/heads/other',trigger:'pull_request'})).status,422);
 assert.equal(creates,0);assert.ok(!data.has('runs/entrypoint-run/intent.json'));
 assert.equal((await request(run)).status,202);assert.equal(creates,1);assert.ok(data.has('runs/entrypoint-run/intent.json'));
 } finally {await rm(dir,{recursive:true,force:true});}
});
