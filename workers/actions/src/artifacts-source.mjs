// Inspect immutable Git truth directly through the Worker binding. Never
// evaluate repository programs in this privileged context.
export async function inspectArtifactSource(artifacts, repoName, sha, allowedRepos) {
  if (!allowedRepos.includes(repoName) || !/^[0-9a-f]{40}$/.test(sha)) throw new Error('source_not_allowed');
  const repo = await artifacts.get(repoName);
  try {
    const commit = await repo.readCommit(sha);
    if (!commit) throw new Error('source_commit_missing');
    const file = await repo.readFile({ref:sha,path:'switchyard.actions.js'});
    let config = null;
    if (file) {
      if (file.size > 65536) throw new Error('source_config_too_large');
      const bytes = await file.arrayBuffer();
      const hash = await crypto.subtle.digest('SHA-256',bytes);
      config = {path:'switchyard.actions.js',bytes:bytes.byteLength,sha256:[...new Uint8Array(hash)].map(x=>x.toString(16).padStart(2,'0')).join('')};
    }
    return {repo:repoName,sha,commit_present:true,config,inspection:'artifacts-worker-binding'};
  } finally {
    repo[Symbol.dispose]?.();
  }
}
