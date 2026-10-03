const {test}=require('node:test'),assert=require('node:assert/strict');
const {parse}=require('../public/assets/js/diff-surface.js');
test('Git diff numbers additions and deletions at hunk coordinates without interpreting markup',()=>{
 const files=parse('diff --git a/file b/file\n--- a/file\n+++ b/file\n@@ -7,2 +7,2 @@\n context\n-removed\n+<script>alert(1)</script>');
 assert.equal(files.length,1);const rows=files[0].lines.slice(-3);assert.deepEqual(rows.map(x=>[x.kind,x.old,x.next]),[['context',7,7],['remove',8,undefined],['add',undefined,8]]);assert.equal(rows[2].text,'<script>alert(1)</script>');
});
