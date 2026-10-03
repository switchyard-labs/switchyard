(() => {
 'use strict';
 const $ = id => document.getElementById(id);
 const roles = [['implementer','Implementation'],['reviewer','Reviews'],['conflict-resolver','Conflict resolution']];
 let preferences = {models:{}, credentials:{}, roles:{}}, providers = [], credentials = [], selected = '';
 async function api(path, options) {
  const r = await fetch(path,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...options});
  const body = await r.json(); if (!r.ok) throw new Error(body.error || 'Could not save preferences'); return body;
 }
 function option(value,label) { const x=document.createElement('option'); x.value=value; x.textContent=label; return x; }
 function providerOptions(select,defaultLabel) { select.replaceChildren(option('',defaultLabel),...providers.map(p=>option(p.id,p.label))); }
 function credentialOptions(select,provider,id) { select.replaceChildren(option('','Select a personal credential'),...credentials.filter(c=>c.provider===provider).map(c=>option(c.id,c.name)));select.value=id||''; }
 function rememberDefault() { if(selected) { preferences.models[selected]=$('agent-model').value.trim(); preferences.credentials[selected]=$('agent-credential').value; } }
 function showDefault() { selected=$('agent-provider').value; $('agent-model').value=preferences.models[selected]||''; credentialOptions($('agent-credential'),selected,preferences.credentials[selected]); }
 async function load() {
  const [config,secrets]=await Promise.all([api('/api/settings/agent'),api('/api/credentials')]);
  preferences=config.preferences; preferences.roles ||= {}; providers=config.providers; credentials=secrets.items||[];
  providerOptions($('credential-provider'),'Choose provider'); $('credential-provider').required=true; providerOptions($('agent-provider'),'No coding provider'); $('agent-provider').value=preferences.provider||'';showDefault();
  $('agent-provider').onchange=()=>{rememberDefault();showDefault()};
  $('agent-runner-status').textContent=config.runner_available ? 'Selections apply to your Agent tasks.' : 'Preferences can be saved. Coding execution requires the server’s sandbox runner to be configured.';
  const host=$('agent-role-preferences');host.replaceChildren();
  for (const [role,label] of roles) {
   const group=document.createElement('fieldset'); const legend=document.createElement('legend');legend.textContent=label;group.append(legend);
   const provider=document.createElement('select');provider.id='role-provider-'+role;provider.setAttribute('aria-label',label+' provider');providerOptions(provider,'Use default');
   const model=document.createElement('input');model.id='role-model-'+role;model.placeholder='Model ID';model.maxLength=200;model.setAttribute('aria-label',label+' model');
   const credential=document.createElement('select');credential.id='role-credential-'+role;credential.setAttribute('aria-label',label+' credential');
   const saved=preferences.roles[role]||{};provider.value=saved.provider||'';model.value=saved.model||'';credentialOptions(credential,provider.value,saved.credential_id);
   provider.onchange=()=>credentialOptions(credential,provider.value,preferences.credentials[provider.value]);
   group.append(provider,model,credential);host.append(group);
  }
 }
 $('agent-preferences-form').onsubmit=async event=>{
  event.preventDefault();rememberDefault();preferences.provider=$('agent-provider').value;
  for(const [role] of roles) { preferences.roles[role]={provider:$('role-provider-'+role).value,model:$('role-model-'+role).value.trim(),credential_id:$('role-credential-'+role).value}; }
  const button=event.target.querySelector('button');button.disabled=true;
  try {await api('/api/settings/agent',{method:'PUT',body:JSON.stringify(preferences)});$('settings-status').textContent='Agent preferences saved';}
  catch(error){$('settings-status').textContent=error.message;}finally{button.disabled=false;}
 };
 document.addEventListener('agent-credentials-changed',()=>load().catch(()=>{}));
 load().catch(error=>{$('agent-runner-status').textContent=error.message;});
})();
