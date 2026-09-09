import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseEnv } from 'node:util';

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
export const workspace = resolve(root, '..');
export const legacy = resolve(workspace, 'kaderisasi-admin-be');

export function testEnvironment(overrides = {}) {
  const configured = parseEnv(readFileSync(resolve(workspace, 'docs/.env.test.be'), 'utf8'));
  return { ...process.env, ...configured, ...overrides };
}
