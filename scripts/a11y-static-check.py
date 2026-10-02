#!/usr/bin/env python3
from pathlib import Path
import re,sys
roots=[Path('public')]
errors=[]
for root in roots:
  for p in root.glob('*.html'):
    s=p.read_text(errors='replace')
    for label,pat in [('title',r'<title>Switchyard</title>'),('viewport',r'name="viewport"'),('main',r'<main\b'),('skip',r'class="skip-link"')]:
      if not re.search(pat,s,re.I): errors.append(f'{p}: missing {label}')
    for m in re.finditer(r'<img\b[^>]*>',s,re.I):
      if not re.search(r'\balt=',m.group(0),re.I): errors.append(f'{p}: image missing alt')
    for m in re.finditer(r'<button\b([^>]*)>(.*?)</button>',s,re.I|re.S):
      attrs,text=m.groups()
      plain=re.sub('<[^>]+>','',text).strip()
      if not plain and 'aria-label=' not in attrs: errors.append(f'{p}: unnamed button')
if errors:
  print('\n'.join(errors)); sys.exit(1)
print('static accessibility gate PASS:',len(list(Path('public').glob('*.html'))),'product pages')
