import {test} from 'node:test';
import assert from 'node:assert/strict';
import {issueBuildAssetGrant,buildAssetRequest} from '../src/build-asset-grants.mjs';
globalThis.FixedLengthStream ??= class extends TransformStream {constructor(){super();}};
const now=1000,secret='fixture-control-secret',scope={run:'run1',repo:'repo1',job:'build',name:'program.zip',sha:'a'.repeat(40),expires:1100,limit:20};
const env=bucket=>({CONTROL_SECRET:secret,ALLOWED_REPOS:'repo1',BACKUP_BUCKET:bucket});
async function request(change={}) {
 const grant=await issueBuildAssetGrant(secret,scope,now);
 return new Request('https://worker.test'+(change.path||grant.path),{method:change.method||'PUT',headers:{Authorization:'Bearer '+(change.token||grant.token),'Content-Length':'3','X-Checksum-SHA256':'b'.repeat(64),...change.headers},...(change.method==='GET'?{}:{body:'abc'})});
}
test('scoped output streams with source binding and conditional integrity',async()=>{
 const response=await buildAssetRequest(await request(),env({put:async(key,body,options)=>{
  assert.match(key,/^build-assets\/repo1\/run1\/build\/[a-f0-9]{64}$/);
  assert.deepEqual(options.onlyIf,{etagDoesNotMatch:'*'});assert.equal(options.sha256,'b'.repeat(64));assert.equal(options.customMetadata.source_sha,scope.sha);
  assert.equal(await new Response(body).text(),'abc');return {size:3};
 }}),now);assert.equal(response.status,200);
});
test('expiry, tampering, another output and non-upload methods fail before storage',async()=>{
 const bucket={put:()=>assert.fail('unauthorized storage')};
 assert.equal((await buildAssetRequest(await request(),env(bucket),1100)).status,401);
 assert.equal((await buildAssetRequest(await request({token:'invalid'}),env(bucket),now)).status,401);
 assert.equal((await buildAssetRequest(await request({path:'/build-assets/another/build/'+'c'.repeat(64)}),env(bucket),now)).status,401);
 assert.equal((await buildAssetRequest(await request({method:'GET'}),env(bucket),now)).status,405);
 assert.equal((await buildAssetRequest(await request(),{...env(bucket),ALLOWED_REPOS:'other'},now)).status,401);
});
test('quota and hash validation occur before storage',async()=>{
 const bucket={put:()=>assert.fail('invalid storage')};
 for(const headers of [{'Content-Length':'21'},{'X-Checksum-SHA256':'invalid'},{'Content-Length':'0'}])assert.equal((await buildAssetRequest(await request({headers}),env(bucket),now)).status,400);
});
test('identical upload retry succeeds but differing bytes cannot replace it',async()=>{
 const old={size:3,customMetadata:{sha256:'b'.repeat(64),source_sha:scope.sha}};
 const bucket={put:async()=>null,head:async()=>old};
 assert.equal((await buildAssetRequest(await request(),env(bucket),now)).status,200);
 old.customMetadata.sha256='c'.repeat(64);
 assert.equal((await buildAssetRequest(await request(),env(bucket),now)).status,409);
});
test('issuer refuses path traversal and unbounded authority',async()=>{
 for(const patch of [{name:'../secret'},{expires:5000},{limit:64*1024*1024+1},{sha:'moving-tag'}])await assert.rejects(issueBuildAssetGrant(secret,{...scope,...patch},now));
});
