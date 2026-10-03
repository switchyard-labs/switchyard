export const maxRequestBytes = 128 * 1024;
export async function boundedBody(request) {
 if (!request.body) return '';
 const reader=request.body.getReader();const chunks=[];let size=0;
 try {for(;;){const {value,done}=await reader.read();if(done)break;size+=value.byteLength;if(size>maxRequestBytes){await reader.cancel();throw new Error('request_too_large');}chunks.push(value);}}
 finally {reader.releaseLock();}
 const bytes=new Uint8Array(size);let offset=0;for(const chunk of chunks){bytes.set(chunk,offset);offset+=chunk.byteLength;}return new TextDecoder('utf-8',{fatal:true}).decode(bytes);
}
export function validateRun(value, namespace, allowedRepos) {
 if (!value || typeof value !== 'object' || value.provider !== 'cloudflare-artifacts' || value.owner !== namespace || value.providerData?.namespace !== namespace) throw new Error('source_not_allowed');
 if (!allowedRepos.includes(value.repo)) throw new Error('repository_not_allowed');
 if (!/^[0-9a-f]{40}$/.test(value.sha) || !/^refs\/(heads|tags)\/[A-Za-z0-9][A-Za-z0-9._/-]{0,250}$/.test(value.ref) || value.ref.includes('..')) throw new Error('invalid_git_identity');
 if (!/^[a-zA-Z0-9_-]{1,90}$/.test(value.run_id) || !/^[0-9a-f]{64}$/.test(value.definition_revision)) throw new Error('invalid_run_identity');
 if (!Array.isArray(value.jobs) || value.jobs.length < 1 || value.jobs.length > 8) throw new Error('invalid_jobs');
 let count = 0; const ids = new Set();
 for (const job of value.jobs) {
  if (!/^[a-zA-Z0-9_-]{1,40}$/.test(job.id) || ids.has(job.id) || !Array.isArray(job.steps) || job.steps.length < 1 || job.steps.length > 16) throw new Error('invalid_job');
  ids.add(job.id); const stepIDs = new Set();
  for (const step of job.steps) {
   if (!/^[a-zA-Z0-9_-]{1,40}$/.test(step.id) || stepIDs.has(step.id) || typeof step.command !== 'string' || !step.command.trim() || step.command.length > 8192 || !Number.isInteger(step.timeout_ms) || step.timeout_ms < 1000 || step.timeout_ms > 120000) throw new Error('invalid_step');
   stepIDs.add(step.id); count++;
  }
 }
 if (count > 32) throw new Error('step_budget_exceeded');
 return value;
}
export async function digest(value) {
 const bytes = new TextEncoder().encode(value);
 return [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(n=>n.toString(16).padStart(2,'0')).join('');
}
export async function signature(secret, timestamp, method, path, body) {
 const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(secret), {name:'HMAC',hash:'SHA-256'}, false, ['sign']);
 return [...new Uint8Array(await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(`${timestamp}\n${method}\n${path}\n${body}`)))].map(n=>n.toString(16).padStart(2,'0')).join('');
}
export function equalSignature(a, b) {
 if (typeof a !== 'string' || a.length !== 64 || b.length !== 64) return false;
 let d=0; for(let i=0;i<64;i++) d|=a.charCodeAt(i)^b.charCodeAt(i); return d===0;
}
