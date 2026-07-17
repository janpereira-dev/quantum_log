#!/usr/bin/env node
'use strict';

const { install } = require('./install');

install().catch((error) => {
  console.error(`qlog npm installer: ${error.message}`);
  process.exitCode = 1;
});
