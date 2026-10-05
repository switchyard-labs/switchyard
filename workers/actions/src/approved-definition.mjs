// A queued run retains its reviewed definition even after another site is configured.
export async function approvedDefinition(bucket, repo, revision) {
 const archived=await bucket.get(`definitions/${repo}/${revision}.json`);
 const current=archived||await bucket.get(`definitions/${repo}.json`);
 const definition=current?await current.json():null;
 if(!definition||definition.revision!==revision||typeof definition.upload_origin!=='string'||new URL(definition.upload_origin).protocol!=='https:')throw Error('Approved upload origin unavailable');
 return definition;
}
export async function saveApprovedDefinition(bucket, repo, definition) {
 const encoded=JSON.stringify(definition);
 await bucket.put(`definitions/${repo}/${definition.revision}.json`,encoded,{onlyIf:{etagDoesNotMatch:'*'}});
 await bucket.put(`definitions/${repo}.json`,encoded);
}
