import {test} from 'node:test';
import assert from 'node:assert/strict';
import {signature} from '../src/protocol.mjs';
import {releaseAssetRequest} from '../src/release-assets.mjs';
globalThis.FixedLengthStream ??= class extends TransformStream {constructor(){super();}};
const path='/release-assets/repo/ast_'+'a'.repeat(24);
async function request(patch={}) {
 const descriptor=Buffer.from(JSON.stringify({size:3,sha256:'b'.repeat(64),content_type:'application/octet-stream',...patch})).toString('base64url');
 const stamp=String(Math.floor(Date.now()/1000));
 return new Request('https://fixture.test'+path,{method:'PUT',headers:{'X-Switchyard-Time':stamp,'X-Switchyard-Asset':descriptor,'X-Switchyard-Signature':await signature('secret',stamp,'PUT',path,descriptor),'Content-Length':'3'},body:'abc'});
}
const env=bucket=>({CONTROL_SECRET:'secret',ALLOWED_REPOS:'repo',BACKUP_BUCKET:bucket});
test('replay completes without consuming its stream',async()=>{
 const response=await releaseAssetRequest(await request(),env({put:async()=>null,head:async()=>({size:3,customMetadata:{sha256:'b'.repeat(64)}})}));
 assert.equal(response.status,200);
});
test('existing content cannot be overwritten',async()=>{
 const response=await releaseAssetRequest(await request(),env({put:async()=>null,head:async()=>({size:4,customMetadata:{sha256:'c'.repeat(64)}})}));assert.equal(response.status,409);
});
test('invalid byte count is rejected before storage',async()=>{
 const response=await releaseAssetRequest(await request({size:4}),env({put:()=>assert.fail('storage accessed')}));assert.equal(response.status,400);
});
test('streamed upload binds integrity and immutable condition',async()=>{
 const response=await releaseAssetRequest(await request(),env({put:async(key,body,options)=>{
 assert.equal(options.sha256,'b'.repeat(64));assert.deepEqual(options.onlyIf,{etagDoesNotMatch:'*'});assert.equal(await new Response(body).text(),'abc');return {size:3};
 }}));assert.equal(response.status,200);
});
