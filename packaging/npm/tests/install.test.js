'use strict';

const assert = require('node:assert/strict');
const { execFileSync, spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { deflateRawSync, gzipSync } = require('node:zlib');
const { archiveName, binaryFromArchive, checksumForArchive, localArtifactPaths, releaseUrls, resolvePlatform, verifySha256 } = require('../scripts/install');

const npmCommand = process.execPath;
const npmCli = process.env.npm_execpath;

function npmArgs(...args) {
  assert.ok(npmCli, 'npm_execpath must identify the current npm CLI');
  return [npmCli, ...args];
}

function tarFile(name, content) {
  const header = Buffer.alloc(512);
  header.write(name);
  header.write(content.length.toString(8).padStart(11, '0') + '\0', 124);
  header[156] = '0'.charCodeAt(0);
  const padding = Buffer.alloc((512 - (content.length % 512)) % 512);
  return gzipSync(Buffer.concat([header, content, padding, Buffer.alloc(1024)]));
}

function zipFile(name, content) {
  const filename = Buffer.from(name);
  const compressed = deflateRawSync(content);
  const local = Buffer.alloc(30);
  local.writeUInt32LE(0x04034b50, 0);
  local.writeUInt16LE(20, 4);
  local.writeUInt16LE(0x08, 6);
  local.writeUInt16LE(8, 8);
  local.writeUInt16LE(filename.length, 26);
  const descriptor = Buffer.alloc(16);
  descriptor.writeUInt32LE(0x08074b50, 0);
  descriptor.writeUInt32LE(compressed.length, 8);
  descriptor.writeUInt32LE(content.length, 12);
  const central = Buffer.alloc(46);
  central.writeUInt32LE(0x02014b50, 0);
  central.writeUInt16LE(20, 4);
  central.writeUInt16LE(20, 6);
  central.writeUInt16LE(0x08, 8);
  central.writeUInt16LE(8, 10);
  central.writeUInt32LE(compressed.length, 20);
  central.writeUInt32LE(content.length, 24);
  central.writeUInt16LE(filename.length, 28);
  const directory = Buffer.concat([central, filename]);
  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(0x06054b50, 0);
  eocd.writeUInt16LE(1, 8);
  eocd.writeUInt16LE(1, 10);
  eocd.writeUInt32LE(directory.length, 12);
  eocd.writeUInt32LE(local.length + filename.length + compressed.length + descriptor.length, 16);
  return Buffer.concat([local, filename, compressed, descriptor, directory, eocd]);
}

test('maps supported platforms to GoReleaser archives', () => {
  const linux = resolvePlatform('linux', 'x64');
  const windows = resolvePlatform('win32', 'arm64');
  assert.equal(archiveName(linux), 'qlog_0.3.2-rc.3_linux_amd64.tar.gz');
  assert.equal(archiveName(windows), 'qlog_0.3.2-rc.3_windows_arm64.zip');
  assert.equal(releaseUrls(linux).archiveUrl, 'https://github.com/janpereira-dev/quantum_log/releases/download/v0.3.2-rc.3/qlog_0.3.2-rc.3_linux_amd64.tar.gz');
  assert.throws(() => resolvePlatform('freebsd', 'x64'), /unsupported platform/);
  assert.throws(() => resolvePlatform('linux', 'ia32'), /unsupported architecture/);
});

test('requires exact SHA-256 manifest entry and verifies content', () => {
  const payload = Buffer.from('verified release');
  const hash = crypto.createHash('sha256').update(payload).digest('hex');
  const archive = 'qlog_0.3.2-rc.3_linux_amd64.tar.gz';
  assert.equal(checksumForArchive(`${hash}  ${archive}\n`, archive), hash);
  assert.throws(() => checksumForArchive(`${hash}  another.tar.gz\n`, archive), /no SHA-256 entry/);
  verifySha256(payload, hash);
  assert.throws(() => verifySha256(payload, '0'.repeat(64)), /verification failed/);
});

test('extracts only qlog from verified TAR release archive', () => {
  assert.deepEqual(binaryFromArchive(tarFile('qlog_0.3.2-rc.3_linux_amd64/qlog', Buffer.from('binary')), resolvePlatform('linux', 'x64')), Buffer.from('binary'));
});

test('extracts only qlog.exe from verified ZIP release archive', () => {
  assert.deepEqual(binaryFromArchive(zipFile('qlog_0.3.2-rc.3_windows_amd64/qlog.exe', Buffer.from('binary')), resolvePlatform('win32', 'x64')), Buffer.from('binary'));
});

test('uses only matching local candidate archive and checksum manifest', () => {
  const directory = path.join(os.tmpdir(), 'qlog-candidate');
  assert.deepEqual(localArtifactPaths(resolvePlatform('linux', 'x64'), directory), {
    archive: path.join(directory, 'qlog_0.3.2-rc.3_linux_amd64.tar.gz'),
    checksums: path.join(directory, 'checksums.txt'),
  });
});

test('postinstall dry-run performs no release download', () => {
  const output = execFileSync(process.execPath, ['scripts/postinstall.js', '--dry-run'], {
    cwd: path.join(__dirname, '..'),
    encoding: 'utf8',
  });
  assert.match(output, /dry-run, no files downloaded or changed/);
  assert.match(output, /releases\/download\/v0\.3\.2-rc\.3\/checksums\.txt/);
});

test('installs from generated local candidate artifact', { skip: !process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR, timeout: 120000 }, async () => {
  const artifactDirectory = process.env.QLOG_INSTALL_LOCAL_ARTIFACT_DIR;
  assert.ok(artifactDirectory, 'QLOG_INSTALL_LOCAL_ARTIFACT_DIR must point to generated candidate artifacts');

  const platform = resolvePlatform();
  const { archive, checksums } = localArtifactPaths(platform, artifactDirectory);
  await fs.access(archive);
  await fs.access(checksums);

  const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'qlog-npm-artifact-'));
  let packagePath;
  try {
    const pack = spawnSync(npmCommand, npmArgs('pack', '--json'), {
      cwd: path.join(__dirname, '..'),
      encoding: 'utf8',
    });
    assert.equal(pack.error, undefined, pack.error?.message);
    assert.equal(pack.status, 0, pack.stderr);
    const packed = JSON.parse(pack.stdout);
    const packageMetadata = Array.isArray(packed) ? packed[0] : Object.values(packed)[0];
    assert.ok(packageMetadata?.filename, `npm pack did not report an output file: ${pack.stdout}`);
    packagePath = path.join(__dirname, '..', packageMetadata.filename);
    const installEnvironment = {
      ...process.env,
      NPM_CONFIG_USERCONFIG: path.join(temporary, '.npmrc'),
      QLOG_INSTALL_LOCAL_ARTIFACT_DIR: artifactDirectory,
    };
    delete installEnvironment.npm_config_allow_scripts;
    delete installEnvironment.NPM_CONFIG_ALLOW_SCRIPTS;
    installEnvironment.npm_config_allow_scripts = '';
    const install = spawnSync(npmCommand, npmArgs('install', '--ignore-scripts', '--prefix', temporary, packagePath), {
      cwd: temporary,
      encoding: 'utf8',
      env: installEnvironment,
    });
    assert.equal(install.error, undefined, install.error?.message);
    assert.equal(install.status, 0, install.stderr);

    const packageRoot = path.join(temporary, 'node_modules', '@janpereira.dev', 'quantum-log');
    const postinstall = spawnSync(process.execPath, ['scripts/postinstall.js'], {
      cwd: packageRoot,
      encoding: 'utf8',
      env: installEnvironment,
    });
    assert.equal(postinstall.error, undefined, postinstall.error?.message);
    assert.equal(postinstall.status, 0, postinstall.stderr);

    const binary = path.join(packageRoot, 'bin', platform.os === 'windows' ? 'qlog.exe' : 'qlog');
    const version = spawnSync(binary, ['--version'], { encoding: 'utf8' });
    assert.equal(version.error, undefined, version.error?.message);
    assert.equal(version.status, 0, version.stderr);
    assert.match(version.stdout, /qlog 0\.3\.2-rc\.2/);
  } finally {
    await fs.rm(temporary, { recursive: true, force: true });
    if (packagePath) await fs.rm(packagePath, { force: true });
  }
});
