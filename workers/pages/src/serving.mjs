const OWNER=/^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$/;
const ID=/^[a-zA-Z0-9_-]{8,100}$/;
const MIME=/^[a-z0-9.+-]+\/[a-z0-9.+-]+(?:; charset=utf-8)?$/i;
const digest=async text=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text))),x=>x.toString(16).padStart(2,'0')).join('');
async function deploymentManifest(bucket,site){
 const object=await bucket.get(`pages/${site.site_id}/${site.deployment_id}/manifest.json`);
 if(!object||!/^[a-f0-9]{64}$/.test(site.manifest_hash||''))throw new Error('deployment_not_ready');
 const raw=await object.text();if(raw.length>4*1024*1024||await digest(raw)!==site.manifest_hash)throw new Error('manifest_integrity_failed');
 const manifest=JSON.parse(raw),prefix=`pages/${site.site_id}/${site.deployment_id}/files/`;
 if(manifest.base_path!==site.base_path||!manifest.files||typeof manifest.files!=='object'||Array.isArray(manifest.files)||Object.keys(manifest.files).length>10000||!manifest.files['/index.html'])throw new Error('invalid_manifest');
 for(const [path,entry] of Object.entries(manifest.files)){
  if(!path.startsWith('/')||sitePath(path)!==path||!entry||!entry.key?.startsWith(prefix)||!/^[a-f0-9]{64}$/.test(entry.sha256||'')||!MIME.test(entry.mime||'')||!Number.isSafeInteger(entry.size)||entry.size<0||entry.size>64*1024*1024)throw new Error('invalid_manifest_entry');
 }
 if(manifest.spa_fallback&&!Object.hasOwn(manifest.files,manifest.spa_fallback))throw new Error('invalid_fallback');
 return manifest;
}
const json=async(bucket,key)=>{const o=await bucket.get(key);return o?JSON.parse(await o.text()):null;};
const missing=()=>new Response('Not found',{status:404,headers:{'Content-Type':'text/plain; charset=utf-8','Cache-Control':'no-store'}});
export function sitePath(raw){
 if(/%(?:2f|5c|00)/i.test(raw)||raw.includes('\\'))throw new Error('invalid_path');
 const decoded=decodeURIComponent(raw);
 if(decoded.split('/').some(x=>x==='.'||x==='..')||/[\x00-\x1f\x7f]/.test(decoded)||decoded.includes('%')||decoded.includes('//'))throw new Error('invalid_path');
 return decoded;
}
export async function serve(request,env){
 if(!['GET','HEAD'].includes(request.method))return new Response('Method not allowed',{status:405,headers:{Allow:'GET, HEAD'}});
 let url,path;try{url=new URL(request.url);path=sitePath(url.pathname);}catch{return new Response('Invalid path',{status:400});}
 const suffix='.'+(env.PAGES_DOMAIN||'switchyard.cx');
 if(!url.hostname.endsWith(suffix))return missing();
 const owner=url.hostname.slice(0,-suffix.length);
 if(!OWNER.test(owner)||owner.includes('.')||['www','api','admin','assets','static','pages','login','signup','sy'].includes(owner))return missing();
 let mapping,site,immutable=false;
 try{
 if(owner.startsWith('dpl-')){
  const id=owner.slice(4);if(!ID.test(id))return missing();
  site=await json(env.PAGES_BUCKET,'pages-previews/'+id+'.json');immutable=true;
  if(!site)return missing();
  if(path==='/'&&site.base_path!=='/')return Response.redirect(url.origin+site.base_path,308);
 }else{
  mapping=await json(env.PAGES_BUCKET,'pages-hosts/'+owner+'.json');if(!mapping)return missing();
  const segment=path.split('/')[1];
  if(Object.hasOwn(mapping.projects||{},segment)){
   site=mapping.projects[segment];if(!site||site.disabled)return missing();
   if(path==='/'+segment)return Response.redirect(url.origin+'/'+encodeURIComponent(segment)+'/'+url.search,308);
  }else site=mapping.root;
 }
 if(!site||site.disabled||!ID.test(site.site_id)||!ID.test(site.deployment_id)||typeof site.base_path!=='string')return missing();
 if(!path.startsWith(site.base_path))return missing();
 const relative='/'+path.slice(site.base_path.length);
 const manifest=await deploymentManifest(env.PAGES_BUCKET,site);
 if(!manifest||manifest.base_path!==site.base_path)return missing();
 let lookup=relative.endsWith('/')?relative+'index.html':relative;
 let entry=manifest.files?.[lookup],status=200;
 if(!entry&&manifest.files?.[relative+'/index.html'])return Response.redirect(url.origin+url.pathname+'/'+url.search,308);
 if(!entry){
  lookup=manifest.spa_fallback&&request.headers.get('Accept')?.includes('text/html')?manifest.spa_fallback:'/404.html';
  entry=manifest.files?.[lookup];status=lookup==='/404.html'?404:200;
 }
 if(!entry)return missing();
 const prefix=`pages/${site.site_id}/${site.deployment_id}/files/`;
 if(!entry.key?.startsWith(prefix)||!/^([a-f0-9]{64})$/.test(entry.sha256)||!MIME.test(entry.mime||''))return missing();
 const etag='"'+entry.sha256+'"';
 const headers=new Headers({'Content-Type':entry.mime,'X-Content-Type-Options':'nosniff','ETag':etag,'Cache-Control':immutable?'public, max-age=31536000, immutable':'public, max-age=0, must-revalidate','Accept-Ranges':'bytes','Referrer-Policy':'strict-origin-when-cross-origin'});
 if(status===200&&request.headers.get('If-None-Match')?.split(',').map(x=>x.trim().replace(/^W\//,'')).includes(etag))return new Response(null,{status:304,headers});
 let range;
 if(status===200&&request.headers.has('Range')&&(!request.headers.has('If-Range')||request.headers.get('If-Range')===etag)){
  const match=/^bytes=(\d*)-(\d*)$/.exec(request.headers.get('Range'));
  if(!match||!match[1]&&!match[2])return new Response(null,{status:416,headers:{'Content-Range':`bytes */${entry.size}`}});
  const start=match[1]?Number(match[1]):Math.max(0,entry.size-Number(match[2]));const end=match[1]?(match[2]?Math.min(Number(match[2]),entry.size-1):entry.size-1):entry.size-1;
  if(!Number.isSafeInteger(start)||!Number.isSafeInteger(end)||start>end||start>=entry.size)return new Response(null,{status:416,headers:{'Content-Range':`bytes */${entry.size}`}});
  range={offset:start,length:end-start+1};headers.set('Content-Range',`bytes ${start}-${end}/${entry.size}`);status=206;
 }
 const object=await env.PAGES_BUCKET.get(entry.key,range?{range}:{});if(!object)return missing();
 headers.set('Content-Length',String(range?.length??entry.size));
 // Never copy arbitrary artifact metadata headers or Set-Cookie.
 return new Response(request.method==='HEAD'?null:object.body,{status,headers});
 }catch{return new Response('Pages temporarily unavailable',{status:503,headers:{'Cache-Control':'no-store'}});}
}
export default {fetch:serve};

export async function promote(bucket,owner,siteKey,target,expectedGeneration,operationID){
 if(!OWNER.test(owner)||owner.startsWith('dpl-')||!ID.test(target.site_id)||!ID.test(target.deployment_id))throw new Error('invalid_mapping');
 const key='pages-hosts/'+owner+'.json',old=await bucket.get(key);
 const map=old?JSON.parse(await old.text()):{generation:0,root:null,projects:{},operations:[]};
 const input=await digest(JSON.stringify([siteKey,target.site_id,target.deployment_id,target.base_path,target.manifest_hash,Boolean(target.disabled)]));
 const prior=(map.operations||[]).find(x=>x.id===operationID);
 if(prior){if(prior.input!==input)throw new Error('operation_input_conflict');return map;}
 if(map.generation!==expectedGeneration)throw new Error('promotion_conflict');
 const mount=siteKey==='/'?'/':'/'+siteKey+'/';
 if(target.base_path!==mount||siteKey!=='/'&&!/^[a-z0-9_-][a-z0-9._-]{0,99}$/.test(siteKey))throw new Error('base_path_mismatch');
 const existing=siteKey==='/'?map.root:map.projects[siteKey];
 if(existing&&existing.site_id!==target.site_id)throw new Error('prefix_owned_by_another_site');
 const manifest=await deploymentManifest(bucket,target);
 if(!manifest||manifest.base_path!==mount)throw new Error('deployment_not_ready');
 for(const entry of Object.values(manifest.files||{})){if((await bucket.head(entry.key))?.size!==entry.size)throw new Error('deployment_incomplete');}
 if(siteKey==='/')map.root=target;else map.projects[siteKey]=target;
 map.generation++;map.operations=[...(map.operations||[]).slice(-99),{id:operationID,input,site:siteKey,deployment:target.deployment_id,previous:existing?.deployment_id||null}];
 const written=await bucket.put(key,JSON.stringify(map),{onlyIf:old?{etagMatches:old.etag}:{etagDoesNotMatch:'*'}});
 if(!written)throw new Error('promotion_conflict');return map;
}
