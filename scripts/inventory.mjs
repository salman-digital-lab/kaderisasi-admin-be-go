import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { readFileSync, readdirSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import vm from 'node:vm';
import { root, legacy } from './env.mjs';
import { inlineValidators } from './inline-validators.mjs';

const require = createRequire(resolve(legacy, 'package.json'));
const baseline=JSON.parse(readFileSync(resolve(root,'docs/BASELINE.json'),'utf8'));
if(execFileSync('git',['rev-parse','HEAD'],{cwd:legacy,encoding:'utf8'}).trim()!==baseline.revision)throw new Error('Adonis baseline changed; review changes and update docs/BASELINE.json explicitly before regenerating contracts.');
const ts = require('typescript');
const compile = text => ts.transpileModule(text, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true } }).outputText;
const top = { children: [], middleware: [], prefix: '' };
let current = top;
function decorate(node) {
  return Object.assign(node, {
    use(value) { this.middleware.push(...(Array.isArray(value) ? value : [value])); return this; },
    prefix(value) { this.pathPrefix = value; return this; },
  });
}
const router = {
  group(callback) {
    const group = decorate({ children: [], middleware: [] });
    current.children.push(group);
    const parent = current;
    current = group;
    callback();
    current = parent;
    return group;
  },
};
for (const method of ['get', 'post', 'put', 'patch', 'delete', 'head', 'options']) {
  router[method] = (path, handler) => {
    const node = decorate({ method: method.toUpperCase(), path, handler, middleware: [] });
    current.children.push(node);
    return node;
  };
}
const middleware = new Proxy({}, { get: (_, name) => options => ({ name, ...options }) });
vm.runInNewContext(compile(readFileSync(resolve(legacy, 'start/routes.ts'), 'utf8')), {
  exports: {}, require: name => name === '#start/kernel' ? { middleware } : router,
});
const routes = [];
function visit(node, prefix = '', inherited = []) {
  const all = [...inherited, ...node.middleware];
  const joined = `${prefix}/${node.pathPrefix ?? ''}`.replace(/\/+$/, '');
  if (node.children) { for (const child of node.children) visit(child, joined, all); return; }
  const controller = Array.isArray(node.handler) ? node.handler[0].toString().match(/#controllers\/([\w_]+)/)?.[1] : 'health';
  routes.push({ method: node.method, path: `${joined}/${node.path}`.replace(/\/+/g, '/').replace(/\/$/, '') || '/', controller, action: Array.isArray(node.handler) ? node.handler[1] : 'health', auth: all.some(x => x.name === 'auth'), trusted_origin: all.some(x => x.name === 'trustedOrigin'), permission: all.find(x => x.name === 'permission')?.permission ?? '' });
}
visit(top);
const cache = new Map();
function loadConstants(name) {
  if (cache.has(name)) return cache.get(name);
  const exports = {};
  vm.runInNewContext(compile(readFileSync(resolve(legacy, `app/constants/${name}.ts`), 'utf8')), { exports, require: path => loadConstants(path.split('/').at(-1)) });
  cache.set(name, exports);
  return exports;
}
for (const folder of ['internal/httpapi', 'internal/auth', 'docs']) mkdirSync(resolve(root, folder), { recursive: true });
writeFileSync(resolve(root, 'internal/httpapi/routes.json'), JSON.stringify(routes, null, 2) + '\n');
const catalog = loadConstants('admin_roles');
writeFileSync(resolve(root, 'internal/auth/roles.json'), JSON.stringify(catalog.ADMIN_ROLES.map(role => catalog.serializeRole(role)), null, 2) + '\n');
writeFileSync(resolve(root, 'internal/auth/permissions.json'), JSON.stringify(loadConstants('permissions').PERMISSION_CODES, null, 2) + '\n');
writeFileSync(resolve(root, 'docs/COMPATIBILITY.md'), '# API compatibility matrix\n\nGenerated from the recorded Adonis baseline. Every row requires success, authorization, invalid input, and missing-resource cases where applicable. Status changes only after tests pass.\n\n| Method | Path | Go owner | Permission | Status |\n|---|---|---|---|---|\n' + routes.map(r => `| ${r.method} | ${r.path} | ${r.controller}.${r.action} | ${r.permission || (r.auth ? 'authenticated' : 'public')} | pending |`).join('\n') + '\n\nJobs: close:registration, clubs:close-registration, clubs:update-visibility — pending.\n');
console.log(`Inventoried ${routes.length} routes and ${catalog.ADMIN_ROLES.length} roles.`);

function schemaRule(kind, args, chain = []) {
  const target = { kind, args, chain };
  return new Proxy(target, { get: (value, key) => {
    if (key === 'toJSON') return () => value;
    if (key in value) return value[key];
    return (...next) => schemaRule(kind, args, [...chain, { method: key, args: next.map(arg => typeof arg === 'function' ? arg.toString() : arg instanceof RegExp ? arg.source : arg) }]);
  }});
}
const vine = new Proxy({}, { get: (_, key) => key === 'compile' ? value => value : (...args) => schemaRule(key,args) });
const clubSource = readFileSync(resolve(legacy,'app/models/club.ts'),'utf8');
const clubTypes = JSON.parse(clubSource.match(/CLUB_TYPES\s*=\s*(\[[\s\S]*?\])/)[1].replaceAll("'",'"'));
const validators = {};
for (const name of readdirSync(resolve(legacy,'app/validators')).filter(x=>x.endsWith('.ts'))) {
  const exports = {};
  vm.runInNewContext(compile(readFileSync(resolve(legacy,'app/validators',name),'utf8')), { exports, require: path => path === '@vinejs/vine' ? {__esModule:true,default:vine} : path === '#models/club' ? {CLUB_TYPES:clubTypes} : loadConstants(path.split('/').at(-1)) });
  Object.assign(validators,exports);
}
mkdirSync(resolve(root,'internal/validation'),{recursive:true});
const inlineExports = {};
vm.runInNewContext(compile(inlineValidators()), {exports:inlineExports,vine});
Object.assign(validators,inlineExports);
writeFileSync(resolve(root,'internal/validation/schemas.json'),JSON.stringify(validators,null,2)+'\n');
console.log(`Inventoried ${Object.keys(validators).length} validators.`);
