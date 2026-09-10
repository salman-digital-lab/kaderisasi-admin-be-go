import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseEnv } from 'node:util';
import storageDNS from './storage-dns.cjs';

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
export const workspace = resolve(root, '..');
export const legacy = resolve(workspace, 'kaderisasi-admin-be');

export function testEnvironment(overrides = {}) {
  const configured = parseEnv(readFileSync(resolve(workspace, 'docs/.env.test.be'), 'utf8'));
  const env = { ...process.env, ...configured, ...overrides };
  if (env.GO_REWRITE_DIRECT_DNS === '1') {
    storageDNS.installStorageDNS(new URL(env.DRIVE_ENDPOINT).hostname);
    env.NODE_OPTIONS = `${env.NODE_OPTIONS ?? ''} --require ${JSON.stringify(resolve(root, 'scripts/storage-dns.cjs'))}`.trim();
    env.GODEBUG = `${env.GODEBUG ? env.GODEBUG + ',' : ''}netdns=go`;
  }
  return env;
}
