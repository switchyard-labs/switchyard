// Called only by an authenticated control-plane request after approved Actions
// completion. An artifact carries content, never site/prefix authority.
const ID=/^[A-Za-z0-9_-]{8,100}$/;
const HASH=/^[a-f0-9]{64}$/;
const MIME=/^[a-z0-9.+-]+\/[a-z0-9.+-]+(?:; charset=utf-8)?$/i;
const hash=async bytes=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),x=>x.toString(16).padStart(2,'0')).join('');
export async function publishArtifact(bucket,bytes,{siteID,deploymentID,basePath,receipt}){
 if(!ID.test(siteID)||!ID.test(deploymentID)||!receipt||!HASH.test(receipt.sha256)||!/^[a-f0-9]{40}$/.test(receipt.source_sha)||receipt.name!=='pages-static.bundle.json'||!receipt.job_id||bytes.byteLength!==receipt.size||bytes.byteLength>64*1024*1024||await hash(bytes)!==receipt.sha256)throw Error('static_receipt_invalid');
 if(!/^\/(?:[a-z0-9_][a-z0-9._-]{0,99}\/)?$/.test(basePath)||basePath==='/.git/')throw Error('static_mount_invalid');
 const bundle=JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(bytes));
 if(bundle.format!=='switchyard-static-v1'||bundle.base_path!==basePath||!Array.isArray(bundle.files)||bundle.files.length<1||bundle.files.length>10000)throw Error('static_bundle_invalid');
 const files={},payloads=[];let total=0;
 // Validate the complete artifact before the first external write.
 for(const entry of bundle.files){
  if(!entry||typeof entry.path!=='string'||!entry.path.startsWith('/')||entry.path.length>1024||/[\\%\x00-\x1f\x7f]/.test(entry.path)||entry.path.slice(1).split('/').some(part=>!part||['.','..','.git'].includes(part))||Object.hasOwn(files,entry.path)||!HASH.test(entry.sha256)||!MIME.test(entry.mime||'')||!Number.isSafeInteger(entry.size)||entry.size<0||entry.size>48*1024*1024||typeof entry.body!=='string'||! /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(entry.body))throw Error('static_entry_invalid');
  const binary=atob(entry.body),content=Uint8Array.from(binary,c=>c.charCodeAt(0));total+=content.byteLength;
  if(content.byteLength!==entry.size||total>48*1024*1024||await hash(content)!==entry.sha256)throw Error('static_content_invalid');
  const key=`pages/${siteID}/${deploymentID}/files/${entry.sha256}`;files[entry.path]={key,sha256:entry.sha256,size:entry.size,mime:entry.mime};payloads.push({key,content,sha256:entry.sha256});
 }
 if(!files['/index.html']||bundle.spa_fallback&&!Object.hasOwn(files,bundle.spa_fallback))throw Error('static_index_invalid');
 const manifest=JSON.stringify({base_path:basePath,files,...(bundle.spa_fallback?{spa_fallback:bundle.spa_fallback}:{})});
 if(new TextEncoder().encode(manifest).byteLength>4*1024*1024)throw Error('static_manifest_limit');
 for(const payload of payloads){
  const written=await bucket.put(payload.key,payload.content,{onlyIf:{etagDoesNotMatch:'*'},sha256:payload.sha256,customMetadata:{sha256:payload.sha256}});
  if(!written){const old=await bucket.get(payload.key);if(!old||await hash(await old.arrayBuffer())!==payload.sha256)throw Error('static_payload_conflict');}
 }
 const manifestHash=await hash(new TextEncoder().encode(manifest)),manifestKey=`pages/${siteID}/${deploymentID}/manifest.json`;
 const written=await bucket.put(manifestKey,manifest,{onlyIf:{etagDoesNotMatch:'*'}});
 if(!written){const old=await bucket.get(manifestKey);if(!old||await hash(await old.arrayBuffer())!==manifestHash)throw Error('static_deployment_conflict');}
 return {site_id:siteID,deployment_id:deploymentID,base_path:basePath,manifest_hash:manifestHash,source_sha:receipt.source_sha,artifact_sha256:receipt.sha256,job_id:receipt.job_id,file_count:payloads.length,total_bytes:total};
}
