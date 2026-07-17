'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const https = require('node:https');
const path = require('node:path');
const { gunzipSync, inflateRawSync } = require('node:zlib');

const VERSION = '0.1.0';
const TAG = `v${VERSION}`;
const RELEASE_BASE = 'https://github.com/janpereira-dev/quantum_log/releases/download';
const MAX_CHECKSUM_BYTES = 1024 * 1024;
const MAX_ARCHIVE_BYTES = 200 * 1024 * 1024;
const MAX_EXTRACTED_BYTES = 200 * 1024 * 1024;

function resolvePlatform(platform = process.platform, arch = process.arch) {
  const os = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[platform];
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!os) throw new Error(`unsupported platform: ${platform}`);
  if (!goarch) throw new Error(`unsupported architecture: ${arch}`);
  return { os, arch: goarch, extension: os === 'windows' ? 'zip' : 'tar.gz' };
}

function archiveName(platform) {
  return `qlog_${VERSION}_${platform.os}_${platform.arch}.${platform.extension}`;
}

function releaseUrls(platform) {
  const archive = archiveName(platform);
  const release = `${RELEASE_BASE}/${TAG}`;
  return {
    archive,
    archiveUrl: `${release}/${archive}`,
    checksumsUrl: `${release}/checksums.txt`,
  };
}

function checksumForArchive(manifest, archive) {
  for (const line of manifest.split(/\r?\n/)) {
    const match = /^([A-Fa-f0-9]{64})  (.+)$/.exec(line);
    if (match && match[2] === archive) return match[1].toLowerCase();
  }
  throw new Error(`checksum manifest has no SHA-256 entry for ${archive}`);
}

function verifySha256(archive, expected) {
  const actual = crypto.createHash('sha256').update(archive).digest('hex');
  const expectedBuffer = Buffer.from(expected, 'hex');
  const actualBuffer = Buffer.from(actual, 'hex');
  if (expectedBuffer.length !== 32 || !crypto.timingSafeEqual(actualBuffer, expectedBuffer)) {
    throw new Error('SHA-256 verification failed for release archive');
  }
}

function download(url, maxBytes, redirects = 3) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (response) => {
      const status = response.statusCode || 0;
      if (status >= 300 && status < 400 && response.headers.location) {
        response.resume();
        if (redirects === 0) return reject(new Error(`too many redirects while downloading ${url}`));
        const redirected = new URL(response.headers.location, url);
        if (redirected.protocol !== 'https:') return reject(new Error(`refusing non-HTTPS download redirect: ${redirected.href}`));
        return resolve(download(redirected, maxBytes, redirects - 1));
      }
      if (status !== 200) {
        response.resume();
        return reject(new Error(`download failed for ${url}: HTTP ${status}`));
      }
      const declaredLength = Number(response.headers['content-length']);
      if (Number.isFinite(declaredLength) && declaredLength > maxBytes) {
        response.resume();
        return reject(new Error(`download exceeds ${maxBytes} byte limit: ${url}`));
      }
      const chunks = [];
      let size = 0;
      response.on('data', (chunk) => {
        size += chunk.length;
        if (size > maxBytes) return request.destroy(new Error(`download exceeds ${maxBytes} byte limit: ${url}`));
        chunks.push(chunk);
      });
      response.on('end', () => resolve(Buffer.concat(chunks)));
      response.on('error', reject);
    });
    request.on('error', reject);
  });
}

function safeArchivePath(name) {
  const normalized = path.posix.normalize(name.replaceAll('\\', '/'));
  if (!name || normalized === '.' || normalized.startsWith('../') || path.posix.isAbsolute(normalized)) {
    throw new Error(`unsafe path in release archive: ${name}`);
  }
  return normalized;
}

function tarEntries(archive) {
  const data = gunzipSync(archive, { maxOutputLength: MAX_EXTRACTED_BYTES });
  const entries = [];
  let offset = 0;
  while (offset + 512 <= data.length) {
    const header = data.subarray(offset, offset + 512);
    if (header.every((byte) => byte === 0)) break;
    const name = header.subarray(0, 100).toString('utf8').replace(/\0.*$/, '');
    const prefix = header.subarray(345, 500).toString('utf8').replace(/\0.*$/, '');
    const sizeText = header.subarray(124, 136).toString('ascii').replace(/\0.*$/, '').trim();
    if (!/^[0-7]*$/.test(sizeText)) throw new Error(`invalid TAR entry size for ${name}`);
    const size = Number.parseInt(sizeText || '0', 8);
    const type = String.fromCharCode(header[156]);
    if (!Number.isSafeInteger(size) || size < 0) throw new Error(`invalid TAR entry size for ${name}`);
    const contentStart = offset + 512;
    const contentEnd = contentStart + size;
    if (contentEnd > data.length) throw new Error(`truncated TAR entry: ${name}`);
    if (type === '\0' || type === '0') {
      entries.push({ name: safeArchivePath(prefix ? `${prefix}/${name}` : name), data: data.subarray(contentStart, contentEnd) });
    }
    offset = contentStart + Math.ceil(size / 512) * 512;
  }
  return entries;
}

function zipEntries(archive) {
  const minimumEocdOffset = Math.max(0, archive.length - 65557);
  let eocdOffset = -1;
  for (let offset = archive.length - 22; offset >= minimumEocdOffset; offset--) {
    if (archive.readUInt32LE(offset) === 0x06054b50) {
      eocdOffset = offset;
      break;
    }
  }
  if (eocdOffset < 0) throw new Error('release ZIP archive has no central directory');
  const entryCount = archive.readUInt16LE(eocdOffset + 10);
  const directorySize = archive.readUInt32LE(eocdOffset + 12);
  let offset = archive.readUInt32LE(eocdOffset + 16);
  const directoryEnd = offset + directorySize;
  if (directoryEnd > archive.length) throw new Error('invalid ZIP central directory');

  const entries = [];
  for (let index = 0; index < entryCount; index++) {
    if (offset + 46 > directoryEnd || archive.readUInt32LE(offset) !== 0x02014b50) throw new Error('invalid ZIP central directory entry');
    const flags = archive.readUInt16LE(offset + 8);
    const method = archive.readUInt16LE(offset + 10);
    const compressedSize = archive.readUInt32LE(offset + 20);
    const uncompressedSize = archive.readUInt32LE(offset + 24);
    const nameLength = archive.readUInt16LE(offset + 28);
    const extraLength = archive.readUInt16LE(offset + 30);
    const commentLength = archive.readUInt16LE(offset + 32);
    const localOffset = archive.readUInt32LE(offset + 42);
    const nextOffset = offset + 46 + nameLength + extraLength + commentLength;
    if ((flags & 0x01) !== 0 || nextOffset > directoryEnd || localOffset + 30 > archive.length || archive.readUInt32LE(localOffset) !== 0x04034b50) {
      throw new Error('unsupported ZIP release archive');
    }
    const name = safeArchivePath(archive.subarray(offset + 46, offset + 46 + nameLength).toString('utf8'));
    const localNameLength = archive.readUInt16LE(localOffset + 26);
    const localExtraLength = archive.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLength + localExtraLength;
    const dataEnd = dataStart + compressedSize;
    if (dataEnd > archive.length) throw new Error(`truncated ZIP entry: ${name}`);
    let data;
    if (method === 0) data = archive.subarray(dataStart, dataEnd);
    else if (method === 8) data = inflateRawSync(archive.subarray(dataStart, dataEnd), { maxOutputLength: MAX_EXTRACTED_BYTES });
    else throw new Error(`unsupported ZIP compression method: ${method}`);
    if (data.length !== uncompressedSize || data.length > MAX_EXTRACTED_BYTES) throw new Error(`invalid ZIP entry size for ${name}`);
    entries.push({ name, data });
    offset = nextOffset;
  }
  return entries;
}

function binaryFromArchive(archive, platform) {
  const binaryName = platform.os === 'windows' ? 'qlog.exe' : 'qlog';
  const entries = platform.extension === 'zip' ? zipEntries(archive) : tarEntries(archive);
  const matches = entries.filter((entry) => path.posix.basename(entry.name) === binaryName);
  if (matches.length !== 1) throw new Error(`release archive must contain exactly one ${binaryName}`);
  return matches[0].data;
}

async function writeBinary(binary, platform) {
  const binaryName = platform.os === 'windows' ? 'qlog.exe' : 'qlog';
  const directory = path.join(__dirname, '..', 'bin');
  const destination = path.join(directory, binaryName);
  const temporary = path.join(directory, `.${binaryName}.${process.pid}.tmp`);
  await fs.mkdir(directory, { recursive: true });
  await fs.writeFile(temporary, binary, { mode: 0o755 });
  await fs.chmod(temporary, 0o755);
  await fs.rm(destination, { force: true });
  await fs.rename(temporary, destination);
}

async function install() {
  const platform = resolvePlatform();
  const urls = releaseUrls(platform);
  if (process.env.QLOG_INSTALL_DRY_RUN === '1' || process.argv.includes('--dry-run')) {
    console.log(`qlog npm installer: platform ${platform.os}/${platform.arch}`);
    console.log(`qlog npm installer: checksums ${urls.checksumsUrl}`);
    console.log(`qlog npm installer: archive ${urls.archiveUrl}`);
    console.log('qlog npm installer: dry-run, no files downloaded or changed');
    return;
  }
  const manifest = (await download(urls.checksumsUrl, MAX_CHECKSUM_BYTES)).toString('utf8');
  const expected = checksumForArchive(manifest, urls.archive);
  const archive = await download(urls.archiveUrl, MAX_ARCHIVE_BYTES);
  verifySha256(archive, expected);
  await writeBinary(binaryFromArchive(archive, platform), platform);
}

module.exports = { VERSION, archiveName, binaryFromArchive, checksumForArchive, install, releaseUrls, resolvePlatform, verifySha256 };
