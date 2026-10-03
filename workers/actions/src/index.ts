import { CIWorkflow, type CiContext, type CiParams, type CloudflareArtifacts, type CiRunnerResult, isCiRunnerFailure } from '@cloudflare/ci';
import { type CiBindings, CiSandbox } from '@cloudflare/ci/worker';
import type { WorkflowEvent, WorkflowStep } from 'cloudflare:workers';
import { validateRun, signature, equalSignature, digest, boundedBody, validateRerunParent, redactSecrets, secretVariants } from './protocol.mjs';
import { inspectArtifactSource } from './artifacts-source.mjs';
export { CiSandbox };
type Step = {id:string; name?:string; command:string; timeout_ms:number};
type Run = CiParams<CloudflareArtifacts> & {run_id:string; definition_revision:string; jobs:{id:string; name?:string; steps:Step[]}[];rerun_of?:string;selected_jobs?:string[]};
type Env = CiBindings & { CONTROL_SECRET:string; ARTIFACTS_NAMESPACE:string; ALLOWED_REPOS:string };
const json = (value:unknown, status=200)=>Response.json(value,{status,headers:{'Cache-Control':'no-store'}});
const privilegedSecrets=(env:Env)=>[env.CONTROL_SECRET,env.CF_TOKEN,env.R2_ACCESS_KEY_ID,env.R2_SECRET_ACCESS_KEY];
const key = (id:string)=>`runs/${id}/manifest.json`;
async function logText(value:string|ReadableStream<Uint8Array>,secrets:string[]):Promise<{text:string;truncated:boolean}> {
 const limit=1024*1024;
 if(typeof value==='string'){const safe=new TextEncoder().encode(redactSecrets(value,secrets));return {text:new TextDecoder().decode(safe.subarray(0,limit),{stream:true}),truncated:safe.byteLength>limit};}
 const reader=value.getReader(); let size=0, text=''; const decoder=new TextDecoder(); let truncated=false;
 try { for(;;) {const {value,done}=await reader.read(); if(done) break; const room=limit-size; text+=decoder.decode(value.subarray(0,Math.max(0,room)),{stream:true});size+=value.length; if(size>limit){truncated=true;await reader.cancel();break;}} text+=decoder.decode(); } finally {reader.releaseLock();}
 const safe=new TextEncoder().encode(redactSecrets(text,secrets));return {text:new TextDecoder().decode(safe.subarray(0,limit),{stream:true}),truncated:truncated||safe.byteLength>limit};
}
export class Actions extends CIWorkflow<CloudflareArtifacts, Env> {
 protected async pipeline(event:WorkflowEvent<CiParams<CloudflareArtifacts>>, step:WorkflowStep, ci:CiContext):Promise<void> {
  let input=event.payload as Run;
  if (!input.run_id) {
   // Native Artifacts triggers are normalized by the official CIWorkflow.
   // Only operator-approved definitions are read, never arbitrary push code
   // evaluated inside the privileged Worker.
   const definition=await step.do('select-definition',async()=>{
    if(input.owner!==this.env.ARTIFACTS_NAMESPACE || !this.env.ALLOWED_REPOS.split(',').includes(input.repo)) throw new Error('Repository not enabled');
    const stored=await this.env.BACKUP_BUCKET.get(`definitions/${input.repo}.json`);
    return stored?await stored.json<{revision:string;jobs:Run['jobs'];refs:string[]}>():null;
   });
   if(!definition || !definition.refs.includes(input.ref)) return;
   input={...input,run_id:event.instanceId,definition_revision:definition.revision,jobs:definition.jobs};
  }
  const run=validateRun(input,this.env.ARTIFACTS_NAMESPACE,this.env.ALLOWED_REPOS.split(',')) as Run;
  if (!('run_id' in event.payload)) {
   const owner=await step.do('claim-native-event',async()=>{
    const identity=await digest(JSON.stringify([run.owner,run.repo,run.ref,run.sha,(event.payload as any).beforeSha||'',run.definition_revision]));
    const receipt=`events/${identity}.json`;
    const claimed=await this.env.BACKUP_BUCKET.put(receipt,JSON.stringify({owner:run.run_id}),{onlyIf:{etagDoesNotMatch:'*'}});
    if(claimed)return run.run_id;
    const existing=await this.env.BACKUP_BUCKET.get(receipt);if(!existing)throw new Error('Native event claim unavailable');
    return (await existing.json<{owner:string}>()).owner;
   });
   if(owner!==run.run_id)return;
  }
  const parent=run.selected_jobs?await step.do('validate-rerun-parent',async()=>{
 const stored=await this.env.BACKUP_BUCKET.get(key(run.rerun_of!));if(!stored)throw new Error('Parent unavailable');const previous=await stored.json<any>();
 return validateRerunParent(run,previous);
 }):null;
  const source=await step.do('inspect-artifacts-source',()=>inspectArtifactSource(this.env.ARTIFACTS,run.repo,run.sha,this.env.ALLOWED_REPOS.split(',')));
  const manifest:{run:Run;source:typeof source;status:string;started_at:string;finished_at?:string;jobs:any[]}={run,source,status:'running',started_at:await step.do('started-at',async()=>new Date().toISOString()),jobs:[]};
  // Workflow step results must be structured-cloneable; R2 HeadResult is not.
  const save=async()=>{await this.env.BACKUP_BUCKET.put(key(run.run_id),JSON.stringify(manifest));};
  await step.do('record-start',()=>save());
  try {
   // Sequential lanes bound this installation to one runner. A job starts from
   // the immutable checkout; chained steps restore only that job's snapshot.
   for(const job of run.jobs) {
    if(parent&&!run.selected_jobs!.includes(job.id)){manifest.jobs.push({...parent.jobs.find((view:any)=>view.id===job.id),reused_from:parent.jobs.find((view:any)=>view.id===job.id).reused_from||run.rerun_of});await save();continue;}
    const view={id:job.id,name:job.name||job.id,status:'running',steps:[] as any[]};manifest.jobs.push(view);let prior:CiRunnerResult|undefined;
    for(const command of job.steps) {
     const label=`${job.id}-${command.id}`;
     const state={id:command.id,name:command.name||command.id,status:'running',started_at:await step.do(`start-${label}`,async()=>new Date().toISOString()),finished_at:'',log_key:`runs/${run.run_id}/${label}.json`,truncated:false};view.steps.push(state);
     const opts={name:label,command:command.command,config:{retries:{limit:0,delay:1000},timeout:command.timeout_ms+10000,commandTimeoutMs:command.timeout_ms,snapshotRetentionSeconds:3600},cloudflareCredentials:false,sourceControlCredentials:false};
     try {prior=await (prior?prior.runner(opts):ci.runner(opts));const stdout=await logText(prior.logs.stdout,privilegedSecrets(this.env)),stderr=await logText(prior.logs.stderr,privilegedSecrets(this.env));state.truncated=stdout.truncated||stderr.truncated;
      await step.do(`logs-${label}`,async()=>{await this.env.BACKUP_BUCKET.put(state.log_key,JSON.stringify({stdout,stderr,kind:'captured',sha:run.sha}));});state.status='succeeded';
     } catch(error) {state.status='failed';view.status='failed';const diagnostic=redactSecrets(isCiRunnerFailure(error)?error.output:'Runner failed; inspect Cloudflare execution',privilegedSecrets(this.env));state.truncated=diagnostic.length>=20000;
      await step.do(`failure-${label}`,async()=>{await this.env.BACKUP_BUCKET.put(state.log_key,JSON.stringify({stdout:{text:'',truncated:false},stderr:{text:diagnostic,truncated:state.truncated},kind:'diagnostic',sha:run.sha}));});throw error;
     } finally {state.finished_at=await step.do(`finish-${label}`,async()=>new Date().toISOString());await save();}
    }
    view.status='succeeded';await save();
   }
   manifest.status='succeeded';
  } catch(error) {manifest.status='failed';throw error;} finally {manifest.finished_at=await step.do('finished-at',async()=>new Date().toISOString());await save();}
 }
}
export default {
 async fetch(request:Request,env:Env):Promise<Response> {
  const url=new URL(request.url);
  if(url.pathname==='/health') return json({ok:true,engine:'cloudflare-ci',protocol:1});
  let body:string;
  try{body=await boundedBody(request);}catch(error){return json({error:'invalid_or_oversized_body'},413);}
  const timestamp=request.headers.get('X-Switchyard-Time')||'',given=request.headers.get('X-Switchyard-Signature')||'';
  if(!env.CONTROL_SECRET||!/^\d{10}$/.test(timestamp)||Math.abs(Date.now()/1000-Number(timestamp))>300||!equalSignature(given,await signature(env.CONTROL_SECRET,timestamp,request.method,url.pathname+url.search,body)))return json({error:'unauthorized'},401);
  try {
   const sourceMatch=url.pathname.match(/^\/source\/([a-zA-Z0-9_-]{1,90})\/([0-9a-f]{40})$/);
   if(sourceMatch && request.method==='GET')return json(await inspectArtifactSource(env.ARTIFACTS,sourceMatch[1],sourceMatch[2],env.ALLOWED_REPOS.split(',')));
   const definitionMatch=url.pathname.match(/^\/definitions\/([a-zA-Z0-9_-]{1,90})$/);
   if(definitionMatch && request.method==='PUT') {
    const repo=definitionMatch[1],definition=JSON.parse(body);
    if(typeof definition.source==='string'&&secretVariants(privilegedSecrets(env)).some(value=>definition.source.includes(value)))return json({error:'credential_values_not_allowed'},422);
    if(typeof definition.source!=='string' || definition.source.length>65536 || definition.revision!==await digest(definition.source) || !Array.isArray(definition.refs) || definition.refs.length>16 || !definition.refs.every((ref:unknown)=>typeof ref==='string' && /^refs\/(heads|tags)\/[a-zA-Z0-9_./-]{1,250}$/.test(ref) && !ref.includes('..')))return json({error:'invalid_definition'},400);
    validateRun({provider:'cloudflare-artifacts',providerData:{namespace:env.ARTIFACTS_NAMESPACE},owner:env.ARTIFACTS_NAMESPACE,repo,sha:'a'.repeat(40),ref:'refs/heads/main',run_id:'definition-validation',definition_revision:definition.revision,jobs:definition.jobs},env.ARTIFACTS_NAMESPACE,env.ALLOWED_REPOS.split(','));
    await env.BACKUP_BUCKET.put(`definitions/${repo}.json`,JSON.stringify(definition));return json({repo,revision:definition.revision});
   }
   if(request.method==='GET' && url.pathname==='/runs') {
    const cursor=url.searchParams.get('cursor')||undefined;
    if(cursor && cursor.length>2048)return json({error:'invalid_cursor'},400);
    const page=await env.BACKUP_BUCKET.list({prefix:'runs/',limit:64,cursor});
    const manifests=await Promise.all(page.objects.filter(object=>object.key.endsWith('/manifest.json')).map(async object=>{const stored=await env.BACKUP_BUCKET.get(object.key);return stored?await stored.json():null;}));
    return json({runs:manifests.filter(Boolean),cursor:page.truncated?page.cursor:null});
   }
   if(request.method==='POST'&&url.pathname==='/dispatch') {
    const run=validateRun(JSON.parse(body),env.ARTIFACTS_NAMESPACE,env.ALLOWED_REPOS.split(',')) as Run;
    const identity=await digest(body),receipt=`runs/${run.run_id}/intent.json`;const existing=await env.BACKUP_BUCKET.get(receipt);
    if(existing && (await existing.json<{digest:string}>()).digest!==identity)return json({error:'run_identity_conflict'},409);
    // Workflow IDs are the durable at-most-one execution boundary. A retry
    // after an ambiguous create reads the same instance instead of a new ID.
    if(!existing){ const created=await env.BACKUP_BUCKET.put(receipt,JSON.stringify({digest:identity,run}),{onlyIf:{etagDoesNotMatch:'*'}}); if(!created){const winner=await env.BACKUP_BUCKET.get(receipt);if(!winner||(await winner.json<{digest:string}>()).digest!==identity)return json({error:'run_identity_conflict'},409);}}
    let instance;
    try {instance=await env.CI_WORKFLOW.create({id:run.run_id,params:run});}catch(error){instance=await env.CI_WORKFLOW.get(run.run_id);await instance.status();}
    return json({id:instance.id,sha:run.sha,definition_revision:run.definition_revision},202);
   }
   const match=url.pathname.match(/^\/runs\/([A-Za-z0-9_-]{1,90})(?:\/(cancel|logs))?$/);
   if(match) {
    const id=match[1],instance=await env.CI_WORKFLOW.get(id);
    if(match[2]==='cancel'&&request.method==='POST'){const current=await instance.status();if(['terminated','complete','errored'].includes(current.status))return json({id,status:current.status});await instance.terminate();return json({id,status:'cancelled'});}
    if(request.method!=='GET')return json({error:'method_not_allowed'},405);
    const status=await instance.status(),manifest=await env.BACKUP_BUCKET.get(key(id));
    if(match[2]==='logs') {
     const label=url.searchParams.get('step')||'';if(!/^[a-zA-Z0-9_-]{1,81}$/.test(label))return json({error:'invalid_step'},400);
     const logs=await env.BACKUP_BUCKET.get(`runs/${id}/${label}.json`);return logs?json(await logs.json()):json({error:'logs_pending'},404);
    }
    return json({id,external_status:status,manifest:manifest?await manifest.json():null});
   }
   return json({error:'not_found'},404);
  }catch(error){return json({error:'cloud_execution_unavailable'},502);}
 }
};
