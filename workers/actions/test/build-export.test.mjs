import {test} from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import http from 'node:http';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import {buildExportScript} from '../src/build-export.mjs';
test('trusted uploader streams exact bytes and rejects a symlink escape without leaking grants',async()=>{
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'switchyard-export-test-'));let requests=0;
 const server=http.createServer(async(req,res)=>{
  requests++;const chunks=[];for await(const chunk of req)chunks.push(chunk);const body=Buffer.concat(chunks);
  assert.equal(body.toString(),'actual build bytes');assert.equal(req.headers['x-checksum-sha256'],createHash('sha256').update(body).digest('hex'));assert.equal(req.headers.authorization,'Bearer disposable-test-grant');
  res.writeHead(200);res.end('{}');
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 try{
  await fs.writeFile(path.join(root,'program.zip'),'actual build bytes');
  const output={path:'program.zip',limit:100,token:'disposable-test-grant',url:`http://127.0.0.1:${server.address().port}/upload`};
  const run=()=>new Promise(resolve=>{
   const child=spawn(process.execPath,['--input-type=module','-e',buildExportScript.replace('"/tmp/ci-source"',JSON.stringify(root))],{env:{...process.env,SWITCHYARD_BUILD_OUTPUTS:JSON.stringify([output])}});let text='';
   child.stdout.on('data',v=>text+=v);child.stderr.on('data',v=>text+=v);child.on('close',code=>resolve({code,text}));
  });
  const success=await run();assert.equal(success.code,0);assert.equal(requests,1);assert.ok(!success.text.includes(output.token));
  await fs.unlink(path.join(root,'program.zip'));await fs.symlink('/etc/hostname',path.join(root,'program.zip'));
  const failure=await run();assert.equal(failure.code,1);assert.equal(requests,1);assert.ok(!failure.text.includes(output.token));
 }finally{await new Promise(resolve=>server.close(resolve));await fs.rm(root,{recursive:true,force:true});}
});
