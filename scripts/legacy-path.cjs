const {resolve} = require('node:path');
module.exports = resolve(process.env.ADONIS_REFERENCE_DIR || resolve(__dirname, '../.artifacts/legacy-admin-be'));
