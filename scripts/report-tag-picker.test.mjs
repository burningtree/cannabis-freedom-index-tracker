import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { chromium } from 'playwright';
let available=true;
try{await fs.access(chromium.executablePath())}catch{available=false}

test('tag picker stays bounded, searches rare tags, preserves selection and keyboard focus', {skip:!available,timeout:30000},async()=>{
 const browser=await chromium.launch({headless:true,executablePath:chromium.executablePath()});
 try{
  const page=await browser.newPage();
  await page.route('**/*',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname.startsWith('/src/'))return route.fulfill({contentType:'application/javascript',body:await fs.readFile(new URL('..'+url.pathname,import.meta.url))});
   return route.fulfill({contentType:'text/html',body:`<div id="tags"><div data-tag-common></div><button data-tag-toggle>Show more</button><div data-tag-panel hidden><input data-tag-search aria-label="Find a tag"><div data-tag-extra></div><p data-tag-empty hidden>No other tags match.</p><button data-tag-more></button></div></div><script type="module">import{initTagPicker}from'/src/scripts/report-tag-picker.js';window.selected='';window.rows=[{g:Array.from({length:3000},(_,i)=>'tag-'+String(i).padStart(4,'0'))}];window.update=initTagPicker(document.querySelector('#tags'),{selected:()=>window.selected,onSelect:tag=>{window.selected=tag;window.update(window.rows)}});window.update(window.rows);</script>`});
  });
  await page.goto('http://tags.test/');
  await page.waitForFunction(()=>window.update);
  assert.equal(await page.locator('[data-tag-common] button').count(),6);
  await page.locator('[data-tag-toggle]').click();
  assert.equal(await page.locator('[data-tag-extra] button').count(),48);
  await page.locator('[data-tag-more]').click();
  assert.equal(await page.locator('[data-tag-extra] button').count(),96);
  await page.getByRole('textbox',{name:'Find a tag'}).fill('tag-2999');
  assert.equal(await page.locator('[data-tag-extra] button').count(),1);
  assert.equal(await page.evaluate(()=>window.selected),'');
  await page.locator('[data-tag-extra] button').click();
  assert.equal(await page.locator('[data-tag-common] [aria-pressed="true"]').textContent(),'#tag-29991');
  assert.equal(await page.evaluate(()=>document.activeElement.dataset.tag),'tag-2999');
  await page.locator('[data-tag-toggle]').click();
  assert.equal(await page.locator('[data-tag-panel]').isVisible(),false);
  assert.equal(await page.locator('[data-tag-common] [data-tag="tag-2999"]').isVisible(),true);
  await page.locator('[data-tag-common] [data-tag="tag-2999"]').click();
  assert.equal(await page.evaluate(()=>window.selected),'');
  assert.equal(await page.locator('[data-tag-common] button').count(),6);
  assert.equal(await page.evaluate(()=>document.activeElement.hasAttribute('data-tag-toggle')),true);
  await page.evaluate(()=>{window.selected='tag-2999';window.rows=[];window.update(window.rows)});
  assert.equal(await page.locator('[data-tag-common] small').textContent(),'0');
  assert.equal(await page.locator('#tags').isVisible(),true);
 }finally{await browser.close()}
});
