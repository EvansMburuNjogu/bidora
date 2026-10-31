import assert from "node:assert/strict";
import { readdir,readFile } from "node:fs/promises";
import test from "node:test";
const root=new URL("../src/app/",import.meta.url);
async function routes(dir){const entries=await readdir(dir,{withFileTypes:true});const nested=await Promise.all(entries.map(entry=>entry.isDirectory()?routes(new URL(`${entry.name}/`,dir)):entry.name==="route.ts"?[new URL(entry.name,dir)]:[]));return nested.flat();}
test("route handlers remain compatible with enabled Cache Components",async()=>{const config=await readFile(new URL("../next.config.ts",import.meta.url),"utf8");assert.match(config,/cacheComponents:\s*true/);for(const path of await routes(root)){const source=await readFile(path,"utf8");assert.doesNotMatch(source,/export\s+const\s+(?:dynamic|revalidate|fetchCache)\s*=/,path.pathname);}});
