import test from 'node:test';
import assert from 'node:assert/strict';
import { submissionDecision, changedReports } from '../src/lib/submission-decision.js';

test('model rejection has its own short title, full explanation and source links', () => {
 const explanation='This source describes a legal change. It does not describe an enforcement event. The country history is the appropriate place for it.';
 const d=submissionDecision({status:'routed_to_cfi',explanation,result:{decision_title:'A change in the law',explanation,sources:[{title:'Original article',url:'https://news.example/story'}]}});
 assert.equal(d.title,'A change in the law');assert.equal(d.details,explanation);assert.equal(d.modelAssessment,null);assert.equal(d.links[0].url,'https://news.example/story');
});
test('service stop is distinguished from a positive model assessment and links the real overlap',()=>{
 const d=submissionDecision({status:'insufficient_evidence',explanation:'The service found overlapping sources.',result:{outcome:'new_event',decision_title:'A supported event',explanation:'The sources support a new sentencing report.',sources:[{title:'Story',url:'https://news.example/shared'}]},validation:{source_overlap:{url:'https://news.example/shared',submission_id:'a'.repeat(32),report_slug:'existing-report'}}},'/tracker/');
 assert.equal(d.title,'Source already used by another case');assert.match(d.modelAssessment,/not a rejection/);assert.doesNotMatch(d.modelAssessment,/sentencing/);assert.equal(d.links[0].url,'/tracker/submission/?id='+'a'.repeat(32));assert.equal(d.links[1].url,'/tracker/report/existing-report/');assert.equal(d.links.filter(x=>x.url==='https://news.example/shared').length,1);
});
test('historical explanations remain visible; unsafe links are omitted; running states are unchanged',()=>{
 const d=submissionDecision({status:'failed',explanation:'Old saved failure',result:{sources:[{url:'javascript:alert(1)'},{url:'https://news.example/safe',title:'<script>plain text</script>'}],existing_report:'../invalid'}});
 assert.equal(d.title,'Failed');assert.equal(d.details,'Old saved failure');assert.equal(d.links.length,1);assert.equal(d.links[0].label,'<script>plain text</script>');
 assert.equal(submissionDecision({status:'researching'}),null);
});
test('a build failure leads with the build error, not the event narrative or its sources',()=>{
 const d=submissionDecision({status:'failed',explanation:'The AI\'s draft did not pass the report build (npm run build). The build reported:\n\ntype: Invalid option',result:{outcome:'existing_case',existing_report:'khust-five-years',explanation:'The page reports the Khust court sentence…',sources:[{title:'Story',url:'https://news.example/story'}]}});
 assert.match(d.details,/Invalid option/);assert.doesNotMatch(d.modelAssessment,/Khust/);assert.deepEqual(d.links.map(l=>l.label),['Related report']);
});
test('changed reports list the existing report and other updated reports once',()=>{
 assert.deepEqual(changedReports({result:{outcome:'additional_source',existing_report:'a-report',report_updates:[{report:'b-report',facts:[{}],summary:'Corrected.'},{report:'a-report',facts:[{}]},{report:'../x'}]}}),[{slug:'a-report',change:'source added'},{slug:'b-report',change:'source added, corrected'}]);
 assert.deepEqual(changedReports({result:{outcome:'new_event'}}),[]);
});
