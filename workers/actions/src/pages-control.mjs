import {publishArtifact} from '../../pages/src/artifact.mjs';
import {promote} from '../../pages/src/serving.mjs';
import {buildAssetKey} from './build-asset-grants.mjs';
export async function pagesControl(path,input,env){
 // Caller signature is verified by the outer Actions control protocol. Runner
 // grants cannot call this endpoint, read artifacts or choose publication maps.
 if(path==='/pages/publish'){
  if(!env.ALLOWED_REPOS.split(',').includes(input.repo)||!/^[A-Za-z0-9_-]{1,90}$/.test(input.run_id||''))throw Error('pages_source_denied');
  const object=await env.BACKUP_BUCKET.get(`runs/${input.run_id}/manifest.json`),manifest=object?await object.json():null;
  if(!manifest||manifest.status!=='succeeded'||manifest.run.repo!==input.repo||manifest.run.sha!==input.source_sha||manifest.run.definition_revision!==input.definition_revision)throw Error('pages_action_not_ready');
  const job=manifest.run.jobs.find(x=>x.id===input.job_id),view=manifest.jobs.find(x=>x.id===input.job_id);
  if(!job?.static||job.static.base_path!==input.base_path||view?.status!=='succeeded'||!view.static)throw Error('pages_static_job_not_ready');
  const receipt=view.static,key=await buildAssetKey({repo:input.repo,run:input.run_id,job:input.job_id,name:'pages-static.bundle.json'});
  if(receipt.key!==key||receipt.source_sha!==input.source_sha||receipt.job_id!==input.job_id)throw Error('pages_receipt_identity');
  const artifact=await env.BACKUP_BUCKET.get(key);if(!artifact||artifact.size!==receipt.size||artifact.customMetadata?.sha256!==receipt.sha256||artifact.customMetadata?.source_sha!==input.source_sha)throw Error('pages_artifact_unavailable');
  const result=await publishArtifact(env.BACKUP_BUCKET,new Uint8Array(await artifact.arrayBuffer()),{siteID:input.site_id,deploymentID:input.deployment_id,basePath:input.base_path,receipt});
  const previewKey=`pages-previews/${input.deployment_id}.json`,serialized=JSON.stringify(result);
  if(!await env.BACKUP_BUCKET.put(previewKey,serialized,{onlyIf:{etagDoesNotMatch:'*'}})){const old=await env.BACKUP_BUCKET.get(previewKey);if(!old||await old.text()!==serialized)throw Error('pages_preview_conflict');}
  return result;
 }
 if(path==='/pages/promote')return promote(env.BACKUP_BUCKET,input.owner,input.project||'/',input.target,input.generation,input.operation_id,{actor:input.actor,action:input.action});
 if(path==='/pages/mapping'){
  if(!/^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$/.test(input.owner||'')||input.owner.startsWith('dpl-'))throw Error('pages_owner_invalid');
  const stored=await env.BACKUP_BUCKET.get(`pages-hosts/${input.owner}.json`);return stored?await stored.json():{generation:0,root:null,projects:{},operations:[]};
 }
 throw Error('pages_operation_unknown');
}
