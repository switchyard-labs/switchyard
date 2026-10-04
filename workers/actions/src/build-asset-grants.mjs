import {signature,equalSignature,digest} from './protocol.mjs';
import {assetLimit} from './release-assets.mjs';

const id=/^[A-Za-z0-9_-]{1,90}$/;
const assetName=/^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$/;
const encode=value=>btoa(JSON.stringify(value)).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
const decode=value=>JSON.parse(atob(value.replace(/-/g,'+').replace(/_/g,'/')));
function validScope(scope,now) {
 return scope&&id.test(scope.run)&&id.test(scope.repo)&&id.test(scope.job)&&assetName.test(scope.name)&&scope.name!=='.'&&scope.name!=='..'&&/^[a-f0-9]{40}$/.test(scope.sha)&&Number.isSafeInteger(scope.expires)&&scope.expires>now&&scope.expires<=now+3600&&Number.isSafeInteger(scope.limit)&&scope.limit>0&&scope.limit<=assetLimit;
}
export async function buildAssetPath(scope) {
 return `/build-assets/${scope.run}/${scope.job}/${await digest(scope.name)}`;
}
export async function issueBuildAssetGrant(secret,scope,now=Math.floor(Date.now()/1000)) {
 if(!secret||!validScope(scope,now))throw new Error('invalid_build_asset_scope');
 const payload=encode(scope),path=await buildAssetPath(scope);
 return {path,token:`${payload}.${await signature(secret,'build-asset','PUT',path,payload)}`};
}

// Build containers receive only this short-lived, single-output capability.
// It cannot read/delete objects, select another run or overwrite prior bytes.
export async function buildAssetRequest(request,env,now=Math.floor(Date.now()/1000)) {
 const path=new URL(request.url).pathname;
 if(!path.startsWith('/build-assets/'))return null;
 if(request.method!=='PUT')return new Response(null,{status:405});
 const token=(request.headers.get('Authorization')||'').replace(/^Bearer /,'');
 if(token.length>2048)return new Response(null,{status:401});
 const parts=token.split('.');let scope;
 try{scope=decode(parts[0]);}catch{return new Response(null,{status:401});}
 if(parts.length!==2||!env.CONTROL_SECRET||!validScope(scope,now)||!env.ALLOWED_REPOS.split(',').includes(scope.repo)||path!==await buildAssetPath(scope)||!equalSignature(parts[1],await signature(env.CONTROL_SECRET,'build-asset','PUT',path,parts[0])))return new Response(null,{status:401});
 const size=Number(request.headers.get('Content-Length')),hash=request.headers.get('X-Checksum-SHA256')||'';
 if(!Number.isSafeInteger(size)||size<1||size>scope.limit||!/^[a-f0-9]{64}$/.test(hash)||!request.body)return new Response(null,{status:400});
 const key=`build-assets/${scope.repo}/${scope.run}/${scope.job}/${await digest(scope.name)}`;
 let count=0;
 const bounded=request.body.pipeThrough(new TransformStream({transform(chunk,controller){count+=chunk.byteLength;if(count>size)throw new Error('size_mismatch');controller.enqueue(chunk);},flush(){if(count!==size)throw new Error('size_mismatch');}}));
 const fixed=new FixedLengthStream(size),abort=new AbortController();
 const piping=bounded.pipeTo(fixed.writable,{signal:abort.signal});piping.catch(()=>{});
 try {
  const object=await env.BACKUP_BUCKET.put(key,fixed.readable,{onlyIf:{etagDoesNotMatch:'*'},sha256:hash,httpMetadata:{contentType:'application/octet-stream'},customMetadata:{sha256:hash,name:scope.name,source_sha:scope.sha}});
  if(!object){abort.abort();const old=await env.BACKUP_BUCKET.head(key);if(old?.size!==size||old.customMetadata?.sha256!==hash||old.customMetadata?.source_sha!==scope.sha)return Response.json({error:'build_asset_exists'},{status:409});}
  else await piping;
  return Response.json({name:scope.name,size,sha256:hash,key,source_sha:scope.sha});
 }catch{abort.abort();return Response.json({error:'build_asset_upload_failed'},{status:422});}
}
