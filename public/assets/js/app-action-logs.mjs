import {appendAnsi} from './action-ansi.mjs';
const connections=new Set();
function closeAll(){for(const stream of connections)stream.close();connections.clear();}
window.addEventListener('pagehide',closeAll);
function bindLogs(event){
  closeAll();
  for(const mount of document.querySelectorAll('.actions-log-mount')) {
    const step=mount.dataset.log,details=mount.closest('details'),url=`${event.detail.apiRoot}/${encodeURIComponent(event.detail.runID)}/logs?step=${encodeURIComponent(step)}`;
    const toolbar=document.createElement('div');toolbar.className='action-log-toolbar';
    const status=document.createElement('span');status.textContent='Captured output';status.setAttribute('role','status');
    const follow=document.createElement('button');follow.className='btn mini';follow.textContent='Pause follow';
    const search=document.createElement('input');search.type='search';search.placeholder='Find in output';search.setAttribute('aria-label','Find in step output');
    const copy=document.createElement('button');copy.className='btn mini';copy.textContent='Copy';
    const reconnect=document.createElement('button');reconnect.className='btn mini';reconnect.textContent='Reconnect';
    const timestamp=document.createElement('button');timestamp.className='btn mini';timestamp.textContent='Capture time';timestamp.setAttribute('aria-pressed','false');
    const download=document.createElement('a');download.className='btn mini';download.textContent='Download';download.href=url+'&download=1';
    toolbar.append(status,search,follow,copy,timestamp,reconnect,download);
    const output=document.createElement('div');output.className='action-log-output';output.tabIndex=0;output.setAttribute('aria-label','Read-only step output');
    const note=document.createElement('p');note.className='muted action-log-note';note.textContent='Output is captured after the step completes. stdout and stderr are grouped separately.';
    mount.append(toolbar,output,note);
    let source=null,cursor=0,following=true,done=false,captureTime='';const lines=[];
    function filter(){const query=search.value.toLowerCase();for(const row of output.children)row.hidden=!!query&&!row.textContent.toLowerCase().includes(query);}
    function connect(){if(done||!details.open)return;source?.close();if(source)connections.delete(source);source=new EventSource(url.replace('/logs?','/logs/stream?')+'&cursor='+cursor);connections.add(source);status.textContent='Waiting for captured output…';
      source.addEventListener('capture',message=>{captureTime=JSON.parse(message.data).captured_at||'';});
      source.addEventListener('line',message=>{const line=JSON.parse(message.data);if(line.id<=cursor||lines.length>=10000)return;cursor=line.id;lines.push(line);const row=document.createElement('div');row.className='action-log-line';const number=document.createElement('span');number.className='action-log-number';number.textContent=line.id;const stream=document.createElement('span');stream.className='action-log-stream';stream.textContent=line.stream;const text=document.createElement('span');appendAnsi(text,line.text);row.append(number,stream,text);output.append(row);filter();if(following)output.scrollTop=output.scrollHeight;status.textContent='Captured output';});
      source.addEventListener('complete',message=>{const info=JSON.parse(message.data);done=true;source.close();connections.delete(source);status.textContent=info.kind==='diagnostic'?'Failure diagnostic':'Capture complete';if(info.truncated)note.textContent+=' Output is truncated; the download contains the provider capture.';});
      source.addEventListener('log_error',()=>{status.textContent='Output unavailable. Reconnect to retry.';source.close();connections.delete(source);});
      source.onerror=()=>{status.textContent='Connection interrupted; reconnecting…';};
    }
    details.addEventListener('toggle',()=>{if(details.open)connect();else{source?.close();connections.delete(source);}});
    reconnect.onclick=()=>{done=false;connect();};search.oninput=filter;
    follow.onclick=()=>{following=!following;follow.textContent=following?'Pause follow':'Follow output';if(following)output.scrollTop=output.scrollHeight;};
    output.addEventListener('scroll',()=>{if(output.scrollHeight-output.scrollTop-output.clientHeight>40){following=false;follow.textContent='Follow output';}});
    timestamp.onclick=()=>{const show=timestamp.getAttribute('aria-pressed')!=='true';timestamp.setAttribute('aria-pressed',String(show));note.textContent=show?(captureTime?'Step capture finished at '+new Date(captureTime).toLocaleString()+'. Per-line timestamps are unavailable.':'Capture time is pending.'):'Output is captured after the step completes. stdout and stderr are grouped separately.';};
    copy.onclick=async()=>{try{await navigator.clipboard.writeText(lines.map(line=>line.text).join('\n'));status.textContent='Output copied';}catch{status.textContent='Clipboard unavailable. Select output to copy.';}};
  }
}
window.addEventListener('switchyard:action-job',bindLogs);
if(window.SwitchyardActions?.runID&&document.querySelector('.actions-log-mount'))bindLogs({detail:window.SwitchyardActions});
