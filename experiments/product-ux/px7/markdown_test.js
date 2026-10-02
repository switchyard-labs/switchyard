global.window=global; global.SwitchyardCode=require('../../../public/assets/js/code-surface.js'); const md=require('../../../public/assets/js/markdown.js');
const src='# Hello\n\n![diagram](images/flow.png)\n\n```go\nfunc main() {}\n```\n\n[bad](javascript:alert(1))';
const out=md.render(src,{resolve:(u,k)=>k==='image'?'/raw/'+u:u});
if(!out.includes('<h1')||!out.includes('/raw/images/flow.png')||!out.includes('tok keyword')||out.includes('href="javascript:')) throw new Error(out);
const raw=md.render('<script>alert(1)</script>'); if(raw.includes('<script>')) throw new Error('raw html passed through');
console.log('PX7 markdown security/render gate PASS');
