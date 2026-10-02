const c = require('../../../public/assets/js/code-surface.js');
const cases = {
  'main.go':'go', 'app.ts':'typescript', 'view.tsx':'tsx', 'main.cpp':'cpp', 'lib.rs':'rust',
  'index.html':'html', 'theme.css':'css', 'package.json':'json', 'config.yaml':'yaml', 'pyproject.toml':'toml',
  'README.md':'markdown', 'script.sh':'shell', 'tool.py':'python', 'schema.sql':'sql', 'template.nift':'nift', 'worker.p':'strut', 'LICENSE':'text'
};
let pass = 0;
for (const [p,want] of Object.entries(cases)) { const got=c.detectLanguage(p); if(got!==want) throw new Error(`${p}: ${got} != ${want}`); pass++; }
const html=c.lines('package main\n// hi\nfunc main() { println("ok") }','go');
if (!html.includes('tok keyword') || !html.includes('tok comment') || !html.includes('tok string') || !html.includes('id="L2"')) throw new Error('highlight output incomplete');
console.log(`PX5 ${pass+1}/${pass+1} PASS`);
