const path = require("path");

const ROOT_DIR = path.resolve(__dirname, "../../..");
const BACKEND_DIR = path.join(ROOT_DIR, "backend");
const FRONTEND_DIR = path.join(ROOT_DIR, "frontend");
const DATA_DIR = path.join(BACKEND_DIR, "data");
const AUDIT_FILE = path.join(DATA_DIR, "audit-store.json");
const PORT = Number(process.env.PORT || 8080);

const MIME_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "application/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".ico": "image/x-icon"
};

module.exports = {
  ROOT_DIR,
  BACKEND_DIR,
  FRONTEND_DIR,
  DATA_DIR,
  AUDIT_FILE,
  PORT,
  MIME_TYPES
};
