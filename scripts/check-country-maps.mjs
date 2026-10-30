// Run after a build: node scripts/check-country-maps.mjs [dist-directory]
// Requires npm run browser-install. Basemap fixtures keep geometry/camera checks deterministic.
import {chromium} from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
const root=path.resolve(process.argv[2] || 'dist');
const output=path.resolve('.cache/country-map-checks');
await fs.mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true,executablePath:chromium.executablePath(),args:['--use-angle=swiftshader','--enable-unsafe-swiftshader']});
try{
 const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(()=>{localStorage.setItem('theme','light');if(!localStorage.getItem('test-default-layers'))localStorage.setItem('map-layers-v2',JSON.stringify({cluster:false,globe:true,area:false,region:false,accession:false}));});
 let current='ru';
 await page.route('**/*',async route=>{
  const u=new URL(route.request().url());
  if(u.hostname==='tiles.openfreemap.org'&&u.pathname.startsWith('/styles/')){
   const shape=JSON.parse(await fs.readFile(`${root}/api/mask/${current}.json`,'utf8'));
   return route.fulfill({json:{version:8,transition:{duration:0},glyphs:'http://tracker.test/fonts/{fontstack}/{range}.pbf',sources:{land:{type:'geojson',data:{type:'Feature',properties:{},geometry:shape.geometry}}},layers:[{id:'background',type:'background',paint:{'background-color':'#e3e5e4'}},{id:'land',type:'fill',source:'land',paint:{'fill-color':'#faf8f2'}}]}});
  }
  if(u.hostname!=='tracker.test')return route.abort();
  if(u.pathname.startsWith('/fonts/'))return route.fulfill({body:Buffer.alloc(0)});
  try{
   const file=path.join(root,u.pathname.endsWith('/')?u.pathname+'index.html':u.pathname);let body=await fs.readFile(file);
   if(file.includes('WorldMap.')&&file.endsWith('.js')){
    body=Buffer.from(body.toString().replace(/new ([\w$]+)\(\{container:/g,'new (new Proxy($1,{construct(T,args){const m=Reflect.construct(T,args);(window.__maps??=[]).push(m);(window.__initialViews??=[]).push({center:m.getCenter().toArray(),zoom:m.getZoom()});return m}}))({container:'));
   }
   if(u.pathname.startsWith('/api/mask/'))await new Promise(r=>setTimeout(r,600));
   return route.fulfill({contentType:file.endsWith('.html')?'text/html':/\.m?js$/.test(file)?'application/javascript':file.endsWith('.css')?'text/css':'application/json',body});
  }catch{return route.abort();}
 });
 for(const width of [1440,390])for(const id of ['ru','ag','ar','au']){
  current=id;await page.setViewportSize({width,height:1000});await page.goto(`http://tracker.test/country/${id}/`);
  await page.waitForFunction(()=>window.__maps?.length);
  const box=page.locator('[data-map]');await box.scrollIntoViewIfNeeded();
  if(id==='ru'&&width===1440){await box.locator('.map-loading').screenshot({path:path.join(output,'preloader.png')});assert(await box.locator('.map-loader-pin').count()===4);}
  await page.waitForFunction(()=>document.querySelector('[data-map]').getAttribute('aria-busy')==='false',{},{timeout:30000});
  const result=await page.evaluate(()=>{const m=window.__maps[0];return{initial:window.__initialViews[0],center:m.getCenter().toArray(),zoom:m.getZoom(),projection:m.getProjection()?.type??'mercator',moving:m.isMoving(),mask:m.getSource('mask').serialize().data.features[1].geometry.type,loading:!!document.querySelector('.map-loading')}});
  assert.deepEqual(result.initial.center,result.center);assert.equal(result.initial.zoom,result.zoom);
  assert.equal(result.projection,'mercator');assert.equal(result.moving,false);assert.equal(result.loading,false);assert.equal(result.mask,'MultiLineString');
  if(id==='ag'||id==='ar')assert(result.center[0]<0);
  if(id==='au')assert(result.center[0]>125&&result.center[0]<140&&result.center[1]<-20);
  if(id==='ru')assert(result.center[0]>90&&result.center[0]<120);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await box.screenshot({path:path.join(output,`${id}-${width}.png`)});
  console.log(id,width,JSON.stringify(result));
 }
 // Reduced motion keeps the loader legible without moving pins or routes.
 await page.emulateMedia({reducedMotion:'reduce'});current='au';
 await page.goto('http://tracker.test/country/au/');
 await page.locator('.map-loader-pin').first().waitFor();
 assert.equal(await page.locator('.map-loader-pin').first().evaluate(el=>getComputedStyle(el).animationName),'none');
 assert.equal(await page.locator('.map-loader-route').evaluate(el=>getComputedStyle(el).animationName),'none');
 await page.waitForFunction(()=>!document.querySelector('.map-loading'));
 // Default Landrace layers must be installed before the map is revealed.
 await page.evaluate(()=>localStorage.removeItem('map-layers-v2'));
 // Override the per-navigation preference fixture for this final check.
 await page.evaluate(()=>{localStorage.setItem('test-default-layers','1');localStorage.setItem('map-layers-v2',JSON.stringify({cluster:false,globe:false,area:true,region:true,accession:true}));});
 current='ru';await page.goto('http://tracker.test/country/ru/');
 await page.waitForFunction(()=>window.__maps?.[0]&&!document.querySelector('.map-loading'),{},{timeout:30000});
 assert.deepEqual(await page.evaluate(()=>['points','areas','regions'].map(key=>!!window.__maps[0].getSource('lr-'+key))),[true,true,true]);
 assert.deepEqual(errors,[]);
 console.log('Reduced motion, initial layers, camera stability and map framing passed.');
}finally{await browser.close();}
