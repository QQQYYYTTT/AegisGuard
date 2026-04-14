const { PORT } = require("./config/constants");
const { ensureAuditFile } = require("./data/auditStore");
const { createServer } = require("./app/createServer");

ensureAuditFile();

createServer().listen(PORT, () => {
  console.log(`AegisGuard backend running at http://localhost:${PORT}`);
});
