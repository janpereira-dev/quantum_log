#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const binary = path.join(__dirname, process.platform === 'win32' ? 'qlog.exe' : 'qlog');
const result = spawnSync(binary, process.argv.slice(2), {
  stdio: 'inherit',
  windowsHide: true,
});

if (result.error) {
  throw result.error;
}

process.exitCode = result.status === null ? 1 : result.status;
