const crypto = require("crypto");

function simpleHash(input) {
  return crypto.createHash("sha256").update(String(input)).digest("hex").slice(0, 16);
}

function signPayload(payload) {
  return `sm2-sim-${crypto.createHmac("sha256", "aegisguard-demo-secret").update(JSON.stringify(payload)).digest("hex").slice(0, 20)}`;
}

module.exports = {
  simpleHash,
  signPayload
};
