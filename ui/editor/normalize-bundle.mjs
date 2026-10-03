import {readFileSync,writeFileSync} from 'node:fs';
const path = '../../public/assets/js/cm6.bundle.js';
writeFileSync(path, readFileSync(path,'utf8').replace(/[ \t]+$/gm,''));
