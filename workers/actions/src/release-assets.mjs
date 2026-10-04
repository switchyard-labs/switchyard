import {signature,equalSignature} from './protocol.mjs';
export const assetLimit=64*1024*1024;
export async function releaseAssetRequest(request,env) {
 const url=new URL(request.url),match=url.pathname.match(/^\/release-assets\/([a-zA-Z0-9_-]{1,90})\/(ast_[a-f0-9]{24})$/);
 if(!match)return null;
 const stamp=request.headers.get('X-Switchyard-Time')||'',given=request.headers.get('X-Switchyard-Signature')||'',descriptor=request.headers.get('X-Switchyard-Asset')||'';
 if(!env.CONTROL_SECRET||!/^\d{10}$/.test(stamp)||Math.abs(Date.now()/1000-Number(stamp))>300||descriptor.length>2048||!equalSignature(given,await signature(env.CONTROL_SECRET,stamp,request.method,url.pathname,descriptor)))return Response.json({error:'unauthorized'},{status:401});
 if(!env.ALLOWED_REPOS.split(',').includes(match[1]))return Response.json({error:'repository_not_enabled'},{status:403});
 const key=`release-assets/${match[1]}/${match[2]}`;
 if(request.method==='GET'){
  const object=await env.BACKUP_BUCKET.get(key);if(!object)return new Response(null,{status:404});
  return new Response(object.body,{headers:{'Content-Type':'application/octet-stream','Content-Length':String(object.size),'X-Checksum-SHA256':object.customMetadata?.sha256||'','Cache-Control':'no-store','X-Content-Type-Options':'nosniff'}});
 }
 if(request.method==='DELETE'){await env.BACKUP_BUCKET.delete(key);return Response.json({deleted:true});}
 if(request.method!=='PUT')return new Response(null,{status:405});
 let metadata;
 try{metadata=JSON.parse(atob(descriptor.replace(/-/g,'+').replace(/_/g,'/')));}catch{return Response.json({error:'invalid_asset'},{status:400});}
 if(!Number.isSafeInteger(metadata.size)||metadata.size<1||metadata.size>assetLimit||!/^[a-f0-9]{64}$/.test(metadata.sha256)||metadata.content_type!=='application/octet-stream'||Number(request.headers.get('Content-Length'))!==metadata.size||!request.body)return Response.json({error:'invalid_asset'},{status:400});
 let size=0;
 const bounded=request.body.pipeThrough(new TransformStream({transform(chunk,controller){size+=chunk.byteLength;if(size>metadata.size)throw new Error('asset_size_mismatch');controller.enqueue(chunk);},flush(){if(size!==metadata.size)throw new Error('asset_size_mismatch');}}));
 const fixed=new FixedLengthStream(metadata.size);
 const abort=new AbortController();
 const piping=bounded.pipeTo(fixed.writable,{signal:abort.signal});
 piping.catch(()=>{});
 try{
  const object=await env.BACKUP_BUCKET.put(key,fixed.readable,{onlyIf:{etagDoesNotMatch:'*'},sha256:metadata.sha256,httpMetadata:{contentType:metadata.content_type},customMetadata:{sha256:metadata.sha256}});
  if(!object){abort.abort();const old=await env.BACKUP_BUCKET.head(key);if(old?.size===metadata.size&&old.customMetadata?.sha256===metadata.sha256)return Response.json({size:old.size,sha256:metadata.sha256,key});return Response.json({error:'asset_exists'},{status:409});}
  await piping;
  return Response.json({size:object.size,sha256:metadata.sha256,key});
 }catch{abort.abort();return Response.json({error:'asset_upload_failed'},{status:422});}
}
