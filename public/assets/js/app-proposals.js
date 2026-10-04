(async function(){
'use strict';
const $=id=>document.getElementById(id);if(!$('proposal-page'))return;
const parts=location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
const base='/'+parts.slice(0,2).map(encodeURIComponent).join('/');
const root='/api/repositories/'+parts.slice(0,2).map(encodeURIComponent).join('/')+'/proposals';
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const types=['Bug','Suggestion','Feature','Improvement','Refactor','Documentation','Performance','Experiment','Question','Other'];
for(const kind of types)for(const id of ['proposal-type','proposal-create-type'])$(id).add(new Option(kind,kind));
$('proposal-repo').href=base;$('proposal-repo').textContent=parts.slice(0,2).join('/');
let page=1,current,canWrite=false,canCreate=false,me='';
function error(e){$('proposal-error').hidden=false;$('proposal-error').textContent=e.message;}
async function api(path='',method='GET',data){const response=await fetch(root+path,{method,headers:{'Content-Type':'application/json'},body:data?JSON.stringify(data):undefined});const value=await response.json();if(!response.ok)throw new Error(value.error||'Request failed');return value;}
async function load(){
 $('proposal-error').hidden=true;
 try{
 if(parts[3]){
 document.querySelector('.work-list-toolbar').hidden=true;
 current=await api('/'+encodeURIComponent(parts[3]));
 const discussion=await api('/'+encodeURIComponent(current.id)+'/comments');
 const links=await api('/'+encodeURIComponent(current.id)+'/links');
 $('proposal-body').innerHTML=`<article style="padding:16px;overflow-wrap:anywhere"><h2>${esc(current.title)}</h2><p>${esc(current.type)} · ${esc(current.state)} ${esc(current.closure_outcome)} · ${esc(current.author_principal)}</p><p class="muted">${esc(current.updated_at)} · ${esc(current.provenance?.source)}</p><pre style="white-space:pre-wrap">${esc(current.description)}</pre><div id="proposal-actions" class="dialog-actions"></div><h3>Evidence</h3><pre style="white-space:pre-wrap;overflow-wrap:anywhere">${esc(JSON.stringify(current.provenance,null,2))}</pre><h3>Linked activity</h3>${links.items.map(e=>`<p>${esc(e.relation)}: ${esc(e.target_kind)} ${esc(e.target_id)}</p>`).join('')}${(current.work_intents||[]).map(e=>`<p>Requested Work: <a href="/work/${encodeURIComponent(e.id)}">${esc(e.input.title)}</a></p>`).join('')}<h3>Discussion</h3>${discussion.items.map(c=>`<div data-comment="${esc(c.id)}" style="padding:12px;border-top:1px solid var(--border)"><strong>${esc(c.author_principal)}</strong> · ${esc(c.created_at)}<p style="white-space:pre-wrap">${c.deleted_at?'Comment removed':esc(c.body)}</p></div>`).join('')}<form id="proposal-comment"><label>Comment<textarea required maxlength="16384" name="body"></textarea></label><button class="btn">Comment</button></form><h3>Activity</h3>${(current.history||[]).map(e=>`<p>${esc(e.kind)} · ${esc(e.actor)} · ${esc(e.at)}</p>`).join('')}</article>`;
 $('proposal-comment').hidden=!me;
 for(const comment of discussion.items){
  if(comment.deleted_at)continue;
  const host=Array.from(document.querySelectorAll('[data-comment]')).find(x=>x.dataset.comment===comment.id);
  if(!host)continue;
  if(comment.author_principal===me){
   const edit=document.createElement('button');edit.className='btn';edit.textContent='Edit';
   edit.onclick=()=>{edit.disabled=true;const form=document.createElement('form');form.className='form-stack';const area=document.createElement('textarea');area.value=comment.body;area.required=true;area.maxLength=16384;const save=document.createElement('button');save.className='btn';save.textContent='Save';const cancel=document.createElement('button');cancel.className='btn';cancel.type='button';cancel.textContent='Cancel';cancel.onclick=()=>{form.remove();edit.disabled=false;};form.append(area,save,cancel);form.onsubmit=async e=>{e.preventDefault();try{await api('/'+current.id+'/comments/'+comment.id,'PATCH',{version:discussion.version,body:area.value});await load();}catch(e){error(e);}};host.append(form);area.focus();};host.append(edit);
  }
  if(comment.author_principal===me||canWrite){const remove=document.createElement('button');remove.className='btn';remove.textContent='Delete';remove.onclick=async()=>{try{await api('/'+current.id+'/comments/'+comment.id,'DELETE',{version:discussion.version});await load();}catch(e){error(e);}};host.append(remove);}
 }
 $('proposal-comment').onsubmit=async e=>{e.preventDefault();try{await api('/'+current.id+'/comments','POST',{version:discussion.version,body:new FormData(e.target).get('body')});await load();}catch(e){error(e);}};
 if(canWrite){
 const actions=$('proposal-actions');
 const linkForm=document.createElement('form');linkForm.className='form-stack';linkForm.innerHTML='<label>Relationship<select name="relation"><option value="related">Related Proposal</option><option value="duplicates">Duplicates Proposal</option><option value="superseded_by">Superseded by Proposal</option><option value="work">Existing Work</option></select></label><label>Target ID<input required name="target_id"></label><button class="btn">Link</button>';
 linkForm.onsubmit=async e=>{e.preventDefault();const values=Object.fromEntries(new FormData(linkForm));try{const graph=await api('/'+current.id+'/links');await api('/'+current.id+'/links','POST',{...values,target_kind:values.relation==='work'?'work':'proposal',version:graph.version});await load();}catch(e){await load();error(e);}};actions.after(linkForm);

 for(const [label,state,outcome] of [['Accept','accepted',''],['Defer','deferred',''],['Reject','closed','rejected'],['Answer','closed','answered'],['Complete','closed','completed'],['Reopen','open','']]){
 if(current.state===state&&current.closure_outcome===outcome||label==='Reopen'&&current.state!=='closed')continue;
 const b=document.createElement('button');b.className='btn';b.textContent=label;b.onclick=async()=>{try{await api('/'+current.id,'PATCH',{version:current.version,state,closure_outcome:outcome});await load();}catch(e){error(e);}};actions.append(b);
 }
 const b=document.createElement('button');b.className='btn';b.textContent='Create Work';const workOperation=Array.from(crypto.getRandomValues(new Uint8Array(16)),x=>x.toString(16).padStart(2,'0')).join('');b.onclick=async()=>{b.disabled=true;try{const key=workOperation;const result=await api('/'+current.id+'/work','POST',{version:current.version,operation_id:key,title:current.title,body:current.description});location.href='/work/'+encodeURIComponent(result.work.id);}catch(e){error(e);b.disabled=false;}};actions.append(b);
 }
 $('proposal-previous').hidden=true;$('proposal-next').hidden=true;$('proposal-count').textContent='';return;
 }
 const query=new URLSearchParams({page,state:$('proposal-state').value,type:$('proposal-type').value,q:$('proposal-query').value,author_principal:$('proposal-author').value,label:$('proposal-label').value,priority:$('proposal-priority').value,provenance:$('proposal-source').value});
 const result=await api('?'+query);
 $('proposal-body').innerHTML=result.items.map(p=>`<a class="work-row" style="display:block;padding:12px" href="${base}/proposals/${encodeURIComponent(p.id)}"><strong>${esc(p.title)}</strong><div class="muted">${esc(p.type)} · ${esc(p.state)} · ${esc(p.author_principal)} · ${esc(p.priority)} · ${esc((p.labels||[]).join(', '))} · ${esc(p.provenance?.source)} · ${esc(p.updated_at)}</div></a>`).join('')||'<p class="loading-state">No proposals match these filters.</p>';
 $('proposal-count').textContent=result.total+' proposals';$('proposal-previous').disabled=page===1;$('proposal-next').disabled=page*result.page_size>=result.total;
 }catch(e){error(e);}
}
$('proposal-new').onclick=()=>$('proposal-dialog').showModal();$('proposal-close').onclick=()=>$('proposal-dialog').close();
$('proposal-form').onsubmit=async e=>{e.preventDefault();try{const result=await api('','POST',Object.fromEntries(new FormData(e.target)));location.href=base+'/proposals/'+encodeURIComponent(result.id);}catch(e){error(e);}};
$('proposal-search').onclick=()=>{page=1;load();};$('proposal-previous').onclick=()=>{page--;load();};$('proposal-next').onclick=()=>{page++;load();};
try{const response=await fetch('/api/auth/me');if(response.ok){const auth=await response.json();me=auth.username||auth.user||'';}const responseMeta=await fetch('/api/repositories/'+parts.slice(0,2).map(encodeURIComponent).join('/'));if(responseMeta.ok){const meta=await responseMeta.json();canWrite=!!meta.can_write;if(meta.visibility==='private'){for(const id of ['proposal-type','proposal-create-type'])$(id).add(new Option('Security','Security'));}}}catch{}
try{const response=await fetch('/api/repositories/'+parts.slice(0,2).map(encodeURIComponent).join('/')+'/proposal-settings');if(response.ok){const settings=await response.json();canCreate=!!settings.can_create;}}catch{}
$('proposal-new').hidden=!canCreate;
await load();
})();
