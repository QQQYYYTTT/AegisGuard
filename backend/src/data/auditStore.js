const fs = require("fs");
const { AUDIT_FILE } = require("../config/constants");

function ensureAuditFile() {
  if (!fs.existsSync(AUDIT_FILE)) {
    fs.writeFileSync(AUDIT_FILE, "[]", "utf8");
  }
}

function readAuditStore() {
  ensureAuditFile();
  try {
    return JSON.parse(fs.readFileSync(AUDIT_FILE, "utf8"));
  } catch {
    return [];
  }
}

function writeAuditStore(items) {
  ensureAuditFile();
  fs.writeFileSync(AUDIT_FILE, JSON.stringify(items, null, 2), "utf8");
}

module.exports = {
  ensureAuditFile,
  readAuditStore,
  writeAuditStore
};
