(() => {
 'use strict';
 window.SwitchyardAgentSelection=async function(role) {
  const response=await fetch('/api/settings/agent',{credentials:'same-origin'});if(!response.ok)throw new Error('Agent settings could not be loaded');
  const config=await response.json(), p=config.preferences;
  const secretResponse=await fetch('/api/credentials',{credentials:'same-origin'});if(!secretResponse.ok)throw new Error('Personal credentials could not be loaded');
  const credentials=(await secretResponse.json()).items||[];
  const saved=p.roles?.[role]?.provider?p.roles[role]:{provider:p.provider,model:p.models[p.provider],credential_id:p.credentials[p.provider]};
  const dialog=document.createElement('dialog');dialog.className='agent-task-dialog';
  const form=document.createElement('form');form.className='form-stack';
  const heading=document.createElement('h2');heading.textContent='Choose model for '+({'implementer':'implementation','reviewer':'review','conflict-resolver':'conflict resolution'}[role]||role);
  const provider=document.createElement('select'),model=document.createElement('input'),credential=document.createElement('select');model.maxLength=200;model.placeholder='Model ID';
  function option(id,label){const e=document.createElement('option');e.value=id;e.textContent=label;return e;}
  provider.append(option('','Use saved task default'),...config.providers.map(x=>option(x.id,x.label)));
  function refresh(){const id=provider.value||saved.provider;credential.replaceChildren(option('','Select personal credential'),...credentials.filter(x=>x.provider===id).map(x=>option(x.id,x.name)));credential.value=(id===saved.provider?saved.credential_id:p.credentials[id])||'';model.value=(id===saved.provider?saved.model:p.models[id])||'';model.disabled=credential.disabled=!id;}
  provider.onchange=refresh;refresh();
  function label(text,input){const e=document.createElement('label');e.append(document.createTextNode(text),input);return e;}
  const description=document.createElement('p');description.className='muted';description.textContent=saved.provider?`Saved default: ${saved.provider} / ${saved.model}`:'No coding provider selected. Configure Agent providers in Settings.';
  const controls=document.createElement('div');controls.className='inline-form';const cancel=document.createElement('button');cancel.type='button';cancel.className='btn';cancel.textContent='Cancel';const run=document.createElement('button');run.className='btn primary';run.textContent='Continue';
  controls.append(cancel,run);form.append(heading,description,label('Provider',provider),label('Model',model),label('Credential',credential),controls);dialog.append(form);document.body.append(dialog);dialog.showModal();
  return new Promise(resolve=>{let settled=false;function finish(value){if(settled)return;settled=true;dialog.close();dialog.remove();resolve(value);}cancel.onclick=()=>finish(null);dialog.oncancel=()=>finish(null);form.onsubmit=event=>{event.preventDefault();const effectiveProvider=provider.value||saved.provider;if(effectiveProvider&&(!model.value.trim()||!credential.value)){description.textContent='Select a model and personal credential.';return;}finish(effectiveProvider?{'X-Switchyard-Agent':JSON.stringify({provider:effectiveProvider,model:model.value.trim(),credential_id:credential.value})}:{});};});
 };
})();
