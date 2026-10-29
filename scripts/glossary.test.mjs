import { test } from "node:test";
import assert from "node:assert/strict";
import { glossaryParts, markdownGlossary } from "../src/lib/glossary.js";

test("glossary aliases match once per entry with Unicode word boundaries and CFI casing", () => {
  const seen = new Set();
  const parts = glossaryParts("Decriminalization differs from trafficking. Trafficking is not decriminalised. THC, thc, aTHC and tetrahydrocannabinol.", seen);
  assert.deepEqual(parts.filter(p=>p.entry).map(p=>p.entry.id), ["decriminalised", "trafficking", "THC"]);
  assert.deepEqual(glossaryParts("decriminalized", seen), [{text:"decriminalized"}]);
  assert.equal(glossaryParts("xtrafficking traffickingé traffickings").some(p=>p.entry),false);
});
test("Markdown terms preserve citations, links, code and headings", () => {
  const element = (tagName,value) => ({type:"element",tagName,properties:{},children:[{type:"text",value}]});
  const tree = {type:"root",children:[element("h2","trafficking"),element("a","trafficking"),element("code","trafficking"),element("sup","THC"),element("p","Drug trafficking brings a mandatory minimum.")]};
  markdownGlossary()(tree);
  for (const item of tree.children.slice(0,4)) assert.equal(item.children[0].type,"text");
  const terms=tree.children[4].children.filter(n=>n.type==="element");
  assert.equal(terms.length,2);
  assert.equal(terms[0].properties.dataTermId,"trafficking");
  assert.equal(terms[0].properties.role,"button");
  assert.ok(terms[0].properties.dataText.length>20);
});

test("CFI atlas terms such as bhang have explanations, aliases and article links", () => {
  const parts=glossaryParts("Bhang is sold here; भांग is its Hindi spelling.");
  const bhang=parts.find(part=>part.entry?.id==="Bhang");
  assert.ok(bhang);
  assert.equal(parts.find(part=>part.entry)?.entry.id,"Bhang");
  assert.ok(bhang.entry.text.length>20);
  assert.equal(bhang.entry.url,"https://cannabisfreedom.fyi/culture/language/bhang/");
  const tree={type:"root",children:[{type:"element",tagName:"p",children:[{type:"text",value:"People used bhang."}]}]};
  markdownGlossary()(tree);
  const link=tree.children[0].children.find(n=>n.tagName==="a");
  assert.equal(link.properties.href,bhang.entry.url);
  assert.ok(link.properties.dataText);
});
