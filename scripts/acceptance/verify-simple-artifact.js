#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

assert.equal(process.argv.length, 3, 'usage: node verify-simple-artifact.js <qlog-simple archive>');
const archive = path.resolve(process.argv[2]);
const sandbox = fs.mkdtempSync(path.join(os.tmpdir(), 'qlog-simple-smoke-'));
const artifact = path.join(sandbox, 'artifact');
const binary = path.join(artifact, process.platform === 'win32' ? 'qlog-simple.exe' : 'qlog-simple');
const legacyHome = path.join(sandbox, 'legacy');
const home = path.join(legacyHome, 'simple');
const source = path.join(sandbox, 'demo-session.jsonl');
const sentinel = Buffer.from('existing legacy ledger must stay unchanged');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const run = (...args) => {
  const result = spawnSync(binary, args, {
    cwd: sandbox,
    env: { ...process.env, QLOG_HOME: legacyHome, CODEX_HOME: path.join(sandbox, 'codex') },
    encoding: 'utf8',
    timeout: 30000,
    windowsHide: true,
  });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  return result.stdout;
};

try {
  fs.mkdirSync(artifact);
  const tar = process.platform === 'win32' ? path.join(process.env.SystemRoot, 'System32', 'tar.exe') : 'tar';
  const extracted = spawnSync(tar, ['-xf', archive, '-C', artifact], { encoding: 'utf8', timeout: 30000, windowsHide: true });
  assert.ifError(extracted.error);
  assert.equal(extracted.status, 0, extracted.stderr);
  assert.ok(!fs.existsSync(path.join(artifact, process.platform === 'win32' ? 'qlog.exe' : 'qlog')), 'simple archive contains the legacy binary');
  const fixture = path.join(artifact, 'fixtures', 'demo-session.jsonl');
  assert.deepEqual(fs.readFileSync(fixture), fs.readFileSync(path.join(__dirname, '..', '..', 'fixtures', 'demo-session.jsonl')));
  fs.mkdirSync(legacyHome);
  fs.writeFileSync(path.join(legacyHome, 'qlog.db'), sentinel);
  fs.copyFileSync(fixture, source);
  const sourceBefore = hash(source);
  assert.match(run('--version'), /^qlog (?!simple-dev\b)\S+ \(commit [0-9a-f]{40}, built [^)]+\)\r?\n$/);
  const imported = JSON.parse(run('import', '--json', source));
  const replay = JSON.parse(run('import', '--json', source));
  assert.equal(imported.files, 1);
  assert.equal(imported.samples, 2);
  assert.equal(imported.skipped, 0);
  assert.equal(replay.samples, 0);
  assert.equal(replay.duplicates, 2);
  const database = path.join(home, 'qlog.db');
  const databaseBefore = hash(database);
  const summary = JSON.parse(run('report', '--json'));
  assert.equal(summary.usage_samples, 2);
  assert.equal(summary.total_tokens, 150);
  assert.equal(summary.cost_quality, 'unavailable');
  assert.equal(summary.rows.length, 1);
  assert.equal(summary.rows[0].project, 'unattributed');
  assert.equal(summary.rows[0].provider, 'openai');
  assert.equal(summary.rows[0].model, 'demo-model');
  assert.equal(summary.rows[0].quality, 'agent_reported');
  assert.match(run(), /150/);
  const csv = run('export');
  assert.match(csv, /,openai,demo-model,150,agent_reported,unavailable/);
  assert.ok(!csv.includes('demo-session.jsonl') && !csv.includes(sandbox), 'CSV exposed the source path');
  assert.equal(hash(database), databaseBefore, 'report/export changed the ledger');
  assert.equal(hash(source), sourceBefore, 'import changed the source file');
  assert.deepEqual(fs.readFileSync(path.join(legacyHome, 'qlog.db')), sentinel);
  assert.ok(!fs.readFileSync(database).includes(Buffer.from('SYNTHETIC_PRIVATE_CONTENT_MUST_NOT_BE_STORED')));
  console.log('PASS simple portable artifact: 2 usage samples, 150 tokens, idempotent import, read-only reports, isolated legacy data');
} finally {
  fs.rmSync(sandbox, { recursive: true, force: true });
}
