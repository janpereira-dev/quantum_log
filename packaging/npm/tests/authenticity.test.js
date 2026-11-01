'use strict';

const assert = require('node:assert/strict');
const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const syncFS = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { gzipSync } = require('node:zlib');
const installer = require('../scripts/install');

// These fixtures test the verification gate; mocked cosign does not prove cryptography.
test('cosign gate verifies exact manifest bytes and pinned workflow identity', async (t) => {
 const manifest = Buffer.from([0xef,0xbb,0xbf,0x61,0x0d,0x0a,0xff]);
 const bundle = Buffer.from('{"synthetic":"bundle"}\r\n');
 let calls = 0;
 t.mock.method(childProcess, 'execFileSync', (command,args) => {
  calls++;
  assert.equal(command,'cosign');
  assert.deepEqual(args.slice(0,2),['verify-blob','--bundle']);
  assert.deepEqual(syncFS.readFileSync(args[2]),bundle);
  assert.deepEqual(syncFS.readFileSync(args.at(-1)),manifest);
  assert.equal(args[args.indexOf('--certificate-oidc-issuer')+1],'https://token.actions.githubusercontent.com');
  assert.equal(args[args.indexOf('--certificate-identity')+1],`https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/v${installer.VERSION}`);
  assert.ok(!args.some(arg => arg.startsWith('--insecure') || arg.startsWith('--ignore')));
  return Buffer.from('fixture accepted');
 });
 await installer.verifyManifest(manifest,bundle);
 assert.equal(calls,1);
});

test('cosign gate fails clearly when verifier is missing or rejects signature', async (t) => {
 for (const code of ['ENOENT','SIGNATURE_INVALID']) {
  const mocked = t.mock.method(childProcess,'execFileSync',() => { const error = new Error('synthetic rejection'); error.code=code; throw error; });
  await assert.rejects(installer.verifyManifest(Buffer.from('manifest'),Buffer.from('{}')),code === 'ENOENT' ? /cosign.*required/i : /signature.*workflow identity.*invalid/i);
  mocked.mock.restore();
 }
});

function syntheticArchive(platform) {
 if (platform.extension === 'zip') return Buffer.from('not an archive; authentication must fail before parsing');
 return gzipSync(Buffer.from('not a TAR archive; authentication must fail before parsing'));
}

for (const scenario of ['missing bundle','invalid signature']) {
 test(`local npm ${scenario} preserves installed binary before extraction`, async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(),'qlog-npm-auth-'));
  t.after(() => fs.rm(root,{recursive:true,force:true}));
  const packageRoot = path.join(root,'package');
  await fs.mkdir(path.join(packageRoot,'scripts'),{recursive:true});
  await fs.mkdir(path.join(packageRoot,'bin'));
  await fs.copyFile(path.join(__dirname,'..','scripts','install.js'),path.join(packageRoot,'scripts','install.js'));
  await fs.copyFile(path.join(__dirname,'..','package.json'),path.join(packageRoot,'package.json'));
  const isolated = require(path.join(packageRoot,'scripts','install.js'));
  const platform = isolated.resolvePlatform();
  const binary = path.join(packageRoot,'bin',platform.os === 'windows' ? 'qlog.exe' : 'qlog');
  await fs.writeFile(binary,'original installed binary');
  const artifactRoot = path.join(root,'artifacts');
  await fs.mkdir(artifactRoot);
  const archive = syntheticArchive(platform);
  const hash = crypto.createHash('sha256').update(archive).digest('hex');
  await fs.writeFile(path.join(artifactRoot,isolated.archiveName(platform)),archive);
  await fs.writeFile(path.join(artifactRoot,'checksums.txt'),`${hash}  ${isolated.archiveName(platform)}\r\n`);
  if (scenario === 'invalid signature') await fs.writeFile(path.join(artifactRoot,'checksums.txt.sigstore.json'),'{}');
  t.mock.method(childProcess,'execFileSync',() => { throw new Error('synthetic signature rejection'); });
  const previous = process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR;
  process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR = artifactRoot;
  try { await assert.rejects(isolated.install(),scenario === 'missing bundle' ? /signature bundle.*missing/i : /signature.*workflow identity.*invalid/i); }
  finally { if (previous === undefined) delete process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR; else process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR=previous; }
  assert.equal(await fs.readFile(binary,'utf8'),'original installed binary');
  assert.deepEqual(await fs.readdir(path.join(packageRoot,'bin')),[path.basename(binary)]);
 });
}
