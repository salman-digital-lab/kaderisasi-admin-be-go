import { createRequire } from 'node:module';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { legacy } from './env.mjs';
const ts=createRequire(resolve(legacy,'package.json'))('typescript');

// Extract only top-level schema declarations, never execute controller code.
export function inlineValidators(){
 const chunks=[];
 for(const file of readdirSync(resolve(legacy,'app/controllers')).filter(x=>x.endsWith('.ts'))){
  const text=readFileSync(resolve(legacy,'app/controllers',file),'utf8');
  const source=ts.createSourceFile(file,text,ts.ScriptTarget.ES2022,true);
  for(const statement of source.statements){
   if(!ts.isVariableStatement(statement))continue;
   for(const declaration of statement.declarationList.declarations){
    if(!declaration.initializer?.getText(source).startsWith('vine.compile('))continue;
    chunks.push(`export const ${declaration.name.getText(source)} = ${declaration.initializer.getText(source)};`);
   }
  }
 }
 return chunks.join('\n');
}
