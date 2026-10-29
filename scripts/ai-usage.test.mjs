import { test } from 'node:test';
import assert from 'node:assert/strict';
import { summarizeAIUsage } from '../src/lib/ai-usage.js';
import { runCost } from '../src/lib/model-cost.js';
const bucket = (extra={}) => ({model:'opencode-go/deepseek-v4.1-flash#max',started_at:1793275200,runs:2,running:1,input_tokens:1000,output_tokens:200,reasoning_tokens:50,cache_read_tokens:400,cache_write_tokens:10,cost_usd:0,...extra});
test('AI usage aggregates models without double counting cache or reasoning, and keeps reported costs',()=>{
 const a=bucket(),b=bucket({cost_usd:0.25,running:0});
 const c=bucket({model:'unknown-model',runs:1,running:0});
 const {rows,totals}=summarizeAIUsage([a,b,c],{price_usd:10,allowance_usd:100});
 assert.equal(rows.length,2);
 assert.equal(totals.runs,5);
 assert.equal(totals.input,4230);
 assert.equal(totals.output,750);
 assert.equal(totals.cached,1200);
 assert.equal(totals.reasoning,150);
 assert.equal(totals.unpriced,1);
 assert.equal(totals.cost,runCost(a)+0.25);
 assert.equal(totals.planCost,totals.cost/10);
});
test('empty usage has zero totals, and a reported cost works for an unknown model',()=>{
 assert.equal(summarizeAIUsage([],null).totals.runs,0);
 const {totals}=summarizeAIUsage([bucket({model:'other',cost_usd:2})],null);
 assert.equal(totals.cost,2);
 assert.equal(totals.unpriced,0);
 assert.equal(totals.planCost,0);
});
