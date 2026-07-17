'use strict';

const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const crypto = require('node:crypto');
const path = require('node:path');
const test = require('node:test');
const { deflateRawSync, gzipSync } = require('node:zlib');
const { archiveName, binaryFromArchive, checksumForArchive, releaseUrls, resolvePlatform, verifySha256 } = require('../scripts/install');

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
  assert.equal(archiveName(linux), 'qlog_0.1.0_linux_amd64.tar.gz');
  assert.equal(archiveName(windows), 'qlog_0.1.0_windows_arm64.zip');
  assert.equal(releaseUrls(linux).archiveUrl, 'https://github.com/janpereira-dev/quantum_log/releases/download/v0.1.0/qlog_0.1.0_linux_amd64.tar.gz');
  assert.throws(() => resolvePlatform('freebsd', 'x64'), /unsupported platform/);
  assert.throws(() => resolvePlatform('linux', 'ia32'), /unsupported architecture/);
});

test('requires exact SHA-256 manifest entry and verifies content', () => {
  const payload = Buffer.from('verified release');
  const hash = crypto.createHash('sha256').update(payload).digest('hex');
  const archive = 'qlog_0.1.0_linux_amd64.tar.gz';
  assert.equal(checksumForArchive(`${hash}  ${archive}\n`, archive), hash);
  assert.throws(() => checksumForArchive(`${hash}  another.tar.gz\n`, archive), /no SHA-256 entry/);
  verifySha256(payload, hash);
  assert.throws(() => verifySha256(payload, '0'.repeat(64)), /verification failed/);
});

test('extracts only qlog from verified TAR release archive', () => {
  assert.deepEqual(binaryFromArchive(tarFile('qlog_0.1.0_linux_amd64/qlog', Buffer.from('binary')), resolvePlatform('linux', 'x64')), Buffer.from('binary'));
});

test('extracts only qlog.exe from verified ZIP release archive', () => {
  assert.deepEqual(binaryFromArchive(zipFile('qlog_0.1.0_windows_amd64/qlog.exe', Buffer.from('binary')), resolvePlatform('win32', 'x64')), Buffer.from('binary'));
});

test('postinstall dry-run performs no release download', () => {
  const output = execFileSync(process.execPath, ['scripts/postinstall.js', '--dry-run'], {
    cwd: path.join(__dirname, '..'),
    encoding: 'utf8',
  });
  assert.match(output, /dry-run, no files downloaded or changed/);
  assert.match(output, /releases\/download\/v0\.1\.0\/checksums\.txt/);
});
