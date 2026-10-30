import { test } from 'node:test';
import assert from 'node:assert/strict';
import { rowTags, hasTag, rankedTags, commonTags } from '../src/lib/report-tags.js';

test('common tags count reports once and rank by prevalence with stable ties',()=>{
 const ranked=rankedTags([{g:['airport','hashish','airport']},{g:['airport','customs']},{g:['hashish']},{tags:['court']},{k:'cet-123456 123456 airport'}]);
 assert.deepEqual(ranked,[{tag:'airport',count:2},{tag:'hashish',count:2},{tag:'court',count:1},{tag:'customs',count:1}]);
 assert.deepEqual(rowTags({}),[]);
});

test('tag selection is exact, independent of report prose and short codes',()=>{
 const row={g:['indoor','cross-border'],k:'cet-123456 123456 indoor cross-border',ti:'An indoor operation'};
 assert(hasTag(row,'indoor'));
 assert(!hasTag(row,'door'));
 assert(!hasTag(row,'border'));
 assert(!hasTag(row,'123456'));
 assert(hasTag(row,''));
});

test('rare and zero-count selected tags stay visible in the collapsed picker',()=>{
 const ranked=Array.from({length:10},(_,i)=>({tag:`tag-${i}`,count:10-i}));
 assert.equal(commonTags(ranked,'').length,6);
 assert.equal(commonTags(ranked,'tag-2').length,6);
 assert.deepEqual(commonTags(ranked,'tag-9').at(-1),{tag:'tag-9',count:1});
 assert.deepEqual(commonTags(ranked,'absent').at(-1),{tag:'absent',count:0});
});
