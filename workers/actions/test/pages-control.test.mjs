import test from 'node:test';import assert from 'node:assert/strict';import {createHash} from 'node:crypto';import {pagesControl} from '../src/pages-control.mjs';import {buildAssetKey} from '../src/build-asset-grants.mjs';import {serve} from '../../pages/src/serving.mjs';
const hash=b=>createHash('sha256').update(b).digest('hex');
class Bucket {
 data=new Map();version=0;
 async put(key,body,options={}){const old=this.data.get(key);if(options.onlyIf?.etagDoesNotMatch==='*'&&old||options.onlyIf?.etagMatches&&old?.etag!==options.onlyIf.etagMatches)return null;const bytes=Buffer.from(body);const record={bytes,size:bytes.length,etag:String(++this.version),customMetadata:options.customMetadata};this.data.set(key,record);return record;}
 async get(key,options={}){const x=this.data.get(key);if(!x)return null;const bytes=options.range?x.bytes.subarray(options.range.offset,options.range.offset+options.range.length):x.bytes;return {...x,body:bytes,text:async()=>x.bytes.toString(),json:async()=>JSON.parse(x.bytes),arrayBuffer:async()=>bytes};}
 async head(key){return this.data.get(key)||null;}
}
async function build(bucket,run,html,status='succeeded'){
 const source='a'.repeat(40),bytes=Buffer.from(html),bundle=Buffer.from(JSON.stringify({format:'switchyard-static-v1',base_path:'/foo.js/',files:[{path:'/index.html',mime:'text/html; charset=utf-8',size:bytes.length,sha256:hash(bytes),body:bytes.toString('base64')}]}));
 const key=await buildAssetKey({repo:'docs',run,job:'pages-project',name:'pages-static.bundle.json'}),receipt={key,name:'pages-static.bundle.json',size:bundle.length,sha256:hash(bundle),source_sha:source,job_id:'pages-project'};
 await bucket.put(key,bundle,{customMetadata:{sha256:receipt.sha256,source_sha:source}});
 await bucket.put(`runs/${run}/manifest.json`,JSON.stringify({status,run:{repo:'docs',sha:source,definition_revision:'rev',jobs:[{id:'pages-project',static:{base_path:'/foo.js/'}}]},jobs:[{id:'pages-project',status,static:receipt}]}));
 return {repo:'docs',run_id:run,job_id:'pages-project',source_sha:source,definition_revision:'rev',site_id:'site_docs1',deployment_id:'dpl_'+hash(run).slice(0,32),base_path:'/foo.js/'};
}
test('Actions receipt to preview, promotion, rollback and idempotent replay',async()=>{
 const bucket=new Bucket(),env={BACKUP_BUCKET:bucket,ALLOWED_REPOS:'docs'},serving={PAGES_BUCKET:bucket};
 const a=await pagesControl('/pages/publish',await build(bucket,'run-a','candidate A'),env);
 assert.deepEqual(await pagesControl('/pages/publish',await build(bucket,'run-a','candidate A'),env),a);
 const preview=`https://dpl-${a.deployment_id.slice(4)}.switchyard.cx/foo.js/`;
 assert.equal(await(await serve(new Request(preview),serving)).text(),'candidate A');
 const op=(target,generation,operation_id,action='promote')=>({owner:'strut-labs',project:'foo.js',target,generation,operation_id,actor:'alice',action});
 await pagesControl('/pages/promote',op(a,0,'promote-a'),env);
 const b=await pagesControl('/pages/publish',await build(bucket,'run-b','candidate B'),env);
 const promoted=await pagesControl('/pages/promote',op(b,1,'promote-b'),env);
 assert.equal(await(await serve(new Request('https://strut-labs.switchyard.cx/foo.js/'),serving)).text(),'candidate B');
 const rolled=await pagesControl('/pages/promote',op(a,2,'rollback-a','rollback'),env);
 assert.equal(await(await serve(new Request('https://strut-labs.switchyard.cx/foo.js/'),serving)).text(),'candidate A');
 assert.equal(rolled.operations.at(-1).actor,'alice');assert.equal(rolled.operations.at(-1).action,'rollback');assert.equal(rolled.projects['foo.js'].source_sha,a.source_sha);
 assert.deepEqual(await pagesControl('/pages/promote',op(a,2,'rollback-a','rollback'),env),rolled);
 await assert.rejects(pagesControl('/pages/promote',op(b,1,'stale'),env));
 assert.equal((await pagesControl('/pages/mapping',{owner:'strut-labs'},env)).generation,3);
 assert.equal(promoted.generation,2);
});
test('failed and cancelled Actions cannot publish even with an uploaded artifact',async()=>{
 for(const status of ['failed','cancelled']){const bucket=new Bucket(),input=await build(bucket,'run-'+status,'unsafe',status);await assert.rejects(pagesControl('/pages/publish',input,{BACKUP_BUCKET:bucket,ALLOWED_REPOS:'docs'}));assert.equal([...bucket.data.keys()].filter(k=>k.startsWith('pages/')).length,0);}
});
