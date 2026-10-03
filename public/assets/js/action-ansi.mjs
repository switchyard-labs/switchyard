// Read-only SGR rendering adapted from Warden's terminal style model.
// Every printable segment becomes a text node; OSC links and terminal controls are discarded.
export function ansiSegments(text) {
  const parts=[];let foreground=null,bold=false;let offset=0;
  const controls=/\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]|\x1b[^\[]|[\x00-\x08\x0b-\x1f\x7f]/g;
  const append=value=>{if(value)parts.push({text:value,classes:[foreground===null?'':`ansi-fg-${foreground}`,bold?'term-bold':''].filter(Boolean).join(' ')})};
  for(const match of text.matchAll(controls)) {
    append(text.slice(offset,match.index));offset=match.index+match[0].length;
    if(/^\x1b\[[\d;]*m$/.test(match[0]))for(const code of (match[0].slice(2,-1)||'0').split(';').map(Number)) {
      if(code===0){foreground=null;bold=false;}else if(code===1)bold=true;else if(code===22)bold=false;
      else if(code===39)foreground=null;else if(code>=30&&code<=37)foreground=code-30;else if(code>=90&&code<=97)foreground=code-90+8;
    }
  }
  append(text.slice(offset));return parts;
}
export function appendAnsi(node,text) {for(const part of ansiSegments(text)){const span=document.createElement('span');span.className=part.classes;span.textContent=part.text;node.append(span);}}
