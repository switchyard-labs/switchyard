import test from 'node:test';
import assert from 'node:assert/strict';
import {inspectArtifactSource} from '../src/artifacts-source.mjs';
const sha='a'.repeat(40);
test('binding reads only the immutable allowed commit and releases capability',async()=>{
 let disposed=false;const calls=[];
 const artifacts={async get(name){calls.push(name);return {async readCommit(hash){calls.push(hash);return {};},async readFile(args){calls.push(args);return new Blob(['export default {}']);},[Symbol.dispose](){disposed=true;}};}};
 const result=await inspectArtifactSource(artifacts,'repo',sha,['repo']);
 assert.deepEqual(calls,['repo',sha,{ref:sha,path:'switchyard.actions.js'}]);
 assert.equal(disposed,true);assert.equal(result.config.sha256.length,64);assert.equal(result.config.bytes,17);
 assert.ok(!JSON.stringify(result).includes('export default'));
 await assert.rejects(()=>inspectArtifactSource(artifacts,'other',sha,['repo']));
 await assert.rejects(()=>inspectArtifactSource(artifacts,'repo','main',['repo']));
});
test('missing commit and oversized configuration fail closed and release capability',async()=>{
 for(const missing of [true,false]){let disposed=false;const artifacts={async get(){return {async readCommit(){return missing?null:{};},async readFile(){return new Blob(['x'.repeat(65537)]);},[Symbol.dispose](){disposed=true;}};}};
 await assert.rejects(()=>inspectArtifactSource(artifacts,'repo',sha,['repo']));assert.equal(disposed,true);}
});
