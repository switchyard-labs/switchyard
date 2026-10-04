// Called only after signed control-request authentication; no container start.
export async function recoverSandbox(input,env,readManifest) {
 if(!input||!['inspect','destroy'].includes(input.operation)||!/^[a-f0-9]{64}$/.test(input.id||'')||!/^[a-z0-9-]{1,16}-[a-f0-9-]{36}$/.test(input.name||'')||!/^[A-Za-z0-9_-]{1,90}$/.test(input.run||''))return {error:'invalid_recovery',status:400};
 const manifest=await readManifest(input.run);
 if(!manifest||!env.ALLOWED_REPOS.split(',').includes(manifest.run?.repo)||!['failed','cancelled'].includes(manifest.status))return {error:'terminal_run_required',status:409};
 const workflow=await env.CI_WORKFLOW.get(input.run),state=await workflow.status();
 if(!['errored','terminated','complete'].includes(state.status))return {error:'workflow_not_terminal',status:409};
 const prefixes=manifest.jobs.flatMap(job=>job.steps.map(step=>(job.id+'-'+step.id).toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-+|-+$/g,'').slice(0,16)+'-'));
 if(!prefixes.some(prefix=>input.name.startsWith(prefix)))return {error:'runner_label_mismatch',status:409};
 const id=env.SANDBOX.idFromName(input.name);
 if(id.toString()!==input.id)return {error:'sandbox_identity_mismatch',status:409};
 if(input.operation==='destroy')await env.SANDBOX.get(id).destroy();
 return {status:200,run:input.run,id:input.id,name:input.name,operation:input.operation};
}
