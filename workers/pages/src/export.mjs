// Trusted local exporter; build jobs do not supply object keys or manifests.
import {open,readdir,realpath,lstat} from 'node:fs/promises';
import {constants} from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
const ID=/^[A-Za-z0-9_-]{8,100}$/;
const MIME={'.html':'text/html; charset=utf-8','.css':'text/css; charset=utf-8','.js':'text/javascript; charset=utf-8','.mjs':'text/javascript; charset=utf-8','.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.gif':'image/gif','.webp':'image/webp','.ico':'image/x-icon','.txt':'text/plain; charset=utf-8','.woff2':'font/woff2','.wasm':'application/wasm','.pdf':'application/pdf'};
export async function exportStatic({workspace,output,siteID,deploymentID,basePath='/',spaFallback,bucket}){
 if(!ID.test(siteID)||!ID.test(deploymentID)||!/^\/(?:[a-z0-9_][a-z0-9._-]{0,99}\/)?$/.test(basePath)||basePath==='/.git/')throw Error('invalid_export_identity');
 if(!output||path.isAbsolute(output)||output.split(/[\\/]/).some(x=>!x||x==='.'||x==='..'||x==='.git')||output.includes('\\'))throw Error('invalid_output_directory');
 const workspaceRoot=await realpath(workspace),root=path.resolve(workspaceRoot,output);
 // Reject symlinks at each ancestor, rather than merely accepting a realpath
 // that happens to resolve back inside the checkout.
 let ancestor=workspaceRoot;for(const component of output.split('/')){ancestor=path.join(ancestor,component);if((await lstat(ancestor)).isSymbolicLink())throw Error('symlink_output');}
 if(await realpath(root)!==root||!root.startsWith(workspaceRoot+path.sep))throw Error('output_outside_workspace');
 const files={},payloads=[];let total=0;
 async function walk(dir,relative=''){
  const entries=(await readdir(dir,{withFileTypes:true})).sort((a,b)=>a.name.localeCompare(b.name));
  for(const entry of entries){
   if(entry.name==='.git'||/[\x00-\x1f\x7f%\\]/.test(entry.name))throw Error('invalid_static_path');
   const name=relative+'/'+entry.name,full=path.join(dir,entry.name);
   if(name.length>1024||name.split('/').length>32)throw Error('static_path_limit');
   if(entry.isSymbolicLink())throw Error('symlink_output');
   if(entry.isDirectory()){await walk(full,name);continue;}
   if(!entry.isFile())throw Error('non_regular_output');
   if(payloads.length>=10000)throw Error('static_file_limit');
   const handle=await open(full,constants.O_RDONLY|constants.O_NOFOLLOW);let bytes;
   try{const before=await handle.stat();if(!before.isFile()||before.size>64*1024*1024||total+before.size>256*1024*1024)throw Error('static_size_limit');bytes=await handle.readFile();const after=await handle.stat();if(before.size!==after.size||before.mtimeMs!==after.mtimeMs||bytes.length!==before.size)throw Error('output_changed_during_export');}finally{await handle.close();}
   total+=bytes.length;const hash=createHash('sha256').update(bytes).digest('hex'),key=`pages/${siteID}/${deploymentID}/files/${hash}`;
   files[name]={key,sha256:hash,size:bytes.length,mime:MIME[path.extname(entry.name).toLowerCase()]||'application/octet-stream'};payloads.push({key,bytes});
  }
 }
 await walk(root);if(!files['/index.html'])throw Error('static_index_required');
 if(spaFallback&&!files[spaFallback])throw Error('invalid_spa_fallback');
 const manifest=JSON.stringify({base_path:basePath,files,...(spaFallback?{spa_fallback:spaFallback}:{})});
 if(Buffer.byteLength(manifest)>4*1024*1024)throw Error('manifest_size_limit');
 // Manifest is written last. Neither partial payloads nor this immutable
 // artifact alter a production pointer.
 for(const payload of payloads)await bucket.put(payload.key,payload.bytes,{onlyIf:{etagDoesNotMatch:'*'}});
 const manifestHash=createHash('sha256').update(manifest).digest('hex');
 const written=await bucket.put(`pages/${siteID}/${deploymentID}/manifest.json`,manifest,{onlyIf:{etagDoesNotMatch:'*'}});
 if(!written)throw Error('deployment_identity_exists');
 return {site_id:siteID,deployment_id:deploymentID,base_path:basePath,manifest_hash:manifestHash,file_count:payloads.length,total_bytes:total};
}
