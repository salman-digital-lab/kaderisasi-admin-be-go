import { spawn, execFileSync } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { root, workspace, legacy, testEnvironment } from './env.mjs';

const revision = execFileSync('git',['rev-parse','--short','HEAD'],{cwd:legacy,encoding:'utf8'}).trim();
const out = resolve(root, `.artifacts/baseline-${revision}`);
await mkdir(out, { recursive: true });
const checks = [
  ['admin-lint', legacy, ['npm', 'run', 'lint']],
  ['admin-types', legacy, ['npm', 'run', 'typecheck']],
  ['admin-unit', legacy, ['node', 'ace', 'test', 'unit']],
  ['web-be-lint', resolve(workspace, 'kaderisasi-web-be'), ['npm', 'run', 'lint']],
  ['web-be-types', resolve(workspace, 'kaderisasi-web-be'), ['npm', 'run', 'typecheck']],
  ['web-be-unit', resolve(workspace, 'kaderisasi-web-be'), ['node', 'ace', 'test', 'unit']],
  ['admin-fe-tests', resolve(workspace, 'kaderisasi-admin-fe'), ['npm', 'test']],
  ['admin-fe-build', resolve(workspace, 'kaderisasi-admin-fe'), ['npm', 'run', 'build']],
  ['admin-fe-lint', resolve(workspace, 'kaderisasi-admin-fe'), ['npm', 'run', 'lint']],
  ['web-fe-lint', resolve(workspace, 'kaderisasi-web-fe'), ['npm', 'run', 'lint']],
  ['web-fe-types', resolve(workspace, 'kaderisasi-web-fe'), ['npm', 'run', 'typecheck']],
  ['web-fe-unit', resolve(workspace, 'kaderisasi-web-fe'), ['npm', 'test']],
];
const results = await Promise.all(checks.map(([name, cwd, [command, ...args]]) => new Promise((done) => {
  const started = Date.now();
  // Read-only database connections also protect against accidental baseline test writes.
  const child = spawn(command, args, { cwd, env: testEnvironment({ PGOPTIONS: '-c default_transaction_read_only=on' }), stdio: ['ignore', 'pipe', 'pipe'] });
  let output = '';
  child.stdout.on('data', chunk => { output += chunk; });
  child.stderr.on('data', chunk => { output += chunk; });
  child.on('error', error => { output += error.message; });
  child.on('close', async code => {
    await writeFile(resolve(out, `${name}.log`), output);
    const result = { name, code, duration_ms: Date.now() - started };
    console.log(JSON.stringify(result));
    done(result);
  });
})));
await writeFile(resolve(out, 'results.json'), JSON.stringify(results, null, 2) + '\n');
if(results.some(result=>result.code!==0))process.exitCode=1;
