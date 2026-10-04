// Compatibility guard for Cloudflare sandbox-sdk issue 928. Remove after an upstream fix.
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
export function patchSandbox(root) {
 const pkg=JSON.parse(readFileSync(new URL('package.json',root),'utf8'));
 if(pkg.version!=='0.12.1')throw new Error('Sandbox compatibility patch requires reviewed version 0.12.1');
 const file=new URL('dist/sandbox-DKG3H156.js',root),source=readFileSync(file,'utf8');
 const hash=createHash('sha256').update(source).digest('hex');
 if(hash===PATCHED)return;
 if(hash!==ORIGINAL)throw new Error('Sandbox source changed; compatibility patch requires review');
 const start=source.indexOf('\tasync onStop() {'),end=source.indexOf('\n\t\tlet hadR2EgressMount',start);
 const chunk=source.slice(start,end),old='\t\tthis.client.disconnect();';
 if(start<0||end<0||chunk.split(old).length!==2)throw new Error('Sandbox stop handler mismatch');
 const next=source.slice(0,start)+chunk.replace(old,'\t\tif (this.transport !== "rpc" || this.client.isWebSocketConnected()) this.client.disconnect();')+source.slice(end);
 if(createHash('sha256').update(next).digest('hex')!==PATCHED)throw new Error('Sandbox patch output mismatch');
 writeFileSync(file,next);
}
const ORIGINAL='4549d0ddd7f4cbd2719945fdb4fc6cab65328a04a0ed548ee20a98f1666999dc';
const PATCHED='1b5b9f2806449599d2fd5a91fb61d8919b0e3a8ecbae5341e860db26859fee91';
if(process.argv[1]===new URL(import.meta.url).pathname)patchSandbox(new URL('../node_modules/@cloudflare/sandbox/',import.meta.url));
