import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root } from './env.mjs';
const report=JSON.parse(readFileSync(resolve(root,`.artifacts/contracts/${process.argv[2]}-report.json`),'utf8'));
function differences(a,b,path=''){
  if(Object.is(a,b))return [];
  if(a&&b&&typeof a==='object'&&typeof b==='object'){
    return [...new Set([...Object.keys(a),...Object.keys(b)])].flatMap(key=>differences(a[key],b[key],`${path}/${key}`));
  }
  return [{path,adonis:a??(a===undefined?'<absent>':null),go:b??(b===undefined?'<absent>':null)}];
}
for(const item of report.differences){
  console.log(item.name);
  const seen=new Set();
  for(const difference of differences(item.baseline,item.candidate)){
    const signature=JSON.stringify({...difference,path:difference.path.replace(/\/\d+(?=\/)/g,'/*')});
    if(!seen.has(signature))console.log(JSON.stringify(difference));
    seen.add(signature);
  }
}
