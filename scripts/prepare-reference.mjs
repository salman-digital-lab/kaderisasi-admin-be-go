import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, legacy, migrations } from './env.mjs';

const revision = JSON.parse(readFileSync(resolve(root, 'docs/BASELINE.json'), 'utf8')).revision;
const run = (command, args, cwd) => execFileSync(command, args, { cwd, stdio: 'inherit' });
if (!existsSync(legacy)) run('git', ['worktree', 'add', '--detach', legacy, revision], migrations);
const actual = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: legacy, encoding: 'utf8' }).trim();
if (actual !== revision) throw new Error('Reference revision differs from docs/BASELINE.json; refusing to overwrite it');
const dirty = execFileSync('git', ['status', '--porcelain'], { cwd: legacy, encoding: 'utf8' }).trim();
if (dirty) throw new Error('Reference checkout contains changes; refusing to use a modified baseline');
run('npm', ['ci', '--ignore-scripts', '--no-audit', '--no-fund'], legacy);
run('npm', ['ci', '--ignore-scripts', '--no-audit', '--no-fund'], resolve(root, 'tests/interop'));
console.log(`Prepared historical Adonis reference ${revision}. Ace migrations run from ${migrations}.`);
