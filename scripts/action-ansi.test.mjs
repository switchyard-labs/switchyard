import {test} from 'node:test';
import assert from 'node:assert/strict';
import {ansiSegments} from '../public/assets/js/action-ansi.mjs';
test('SGR color and bold reset without creating markup',()=>{const parts=ansiSegments('\x1b[31;1m<script>alert(1)</script>\x1b[0m plain');assert.equal(parts[0].text,'<script>alert(1)</script>');assert.equal(parts[0].classes,'ansi-fg-1 term-bold');assert.equal(parts[1].classes,'');});
test('OSC links and terminal controls never become links or commands',()=>{assert.equal(ansiSegments('\x1b]8;;https://evil.invalid\x07label\x1b]8;;\x07\x1b[2J\x00').map(x=>x.text).join(''),'label');});
test('extended colour parameters do not become independent SGR commands',()=>{assert.equal(ansiSegments('\u001b[38;2;1;31;1mtext\u001b[0m')[0].classes,'');assert.equal(ansiSegments('\u001b[38;5;31mtext')[0].classes,'');});
