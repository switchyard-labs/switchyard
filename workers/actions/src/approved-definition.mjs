// A queued run retains its reviewed definition even after another site is configured.
export async function approvedDefinition(bucket, repo, revision) {
 const archived=await bucket.get(`definitions/${repo}/${revision}.json`);
 const current=archived||await bucket.get(`definitions/${repo}.json`);
 const definition=current?await current.json():null;
 if(!definition||definition.revision!==revision||typeof definition.upload_origin!=='string'||new URL(definition.upload_origin).protocol!=='https:')throw Error('Approved upload origin unavailable');
 return definition;
}
export async function saveApprovedDefinition(bucket, repo, definition) {
 const encoded=JSON.stringify(definition);
 await bucket.put(`definitions/${repo}/${definition.revision}.json`,encoded,{onlyIf:{etagDoesNotMatch:'*'}});
 await bucket.put(`definitions/${repo}.json`,encoded);
}

// Compare values independently of JSON member order used by Go/control clients.
const canonical=value=>JSON.stringify(value,(_,v)=>v&&typeof v==='object'&&!Array.isArray(v)?Object.fromEntries(Object.keys(v).sort().map(k=>[k,v[k]])):v);
export async function validateApprovedRun(bucket,run){
 const definition=await approvedDefinition(bucket,run.repo,run.definition_revision);
 if(!Array.isArray(definition.refs)||!definition.refs.includes(run.ref))throw Error('ref_not_approved');
 if(canonical(definition.jobs)!==canonical(run.jobs)||canonical(definition.release||null)!==canonical(run.release||null))throw Error('definition_input_conflict');
 return definition;
}
