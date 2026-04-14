const { collectBody, sendJson } = require("../lib/http");
const { issueRequireToken, verifyRequireToken } = require("../services/authService");
const { decideGate } = require("../services/gateService");
const { runMemorySandbox } = require("../services/sandboxService");
const { buildAuditEntry, persistAudit, listAudit, clearAudit } = require("../services/auditService");
const { listAgents, listAttackFamilies, listExperimentLayers, getExperimentPlan, getScenarioTemplates } = require("../services/experimentService");

async function handleApiRequest(req, res, pathname) {
  if (req.method === "GET" && pathname === "/api/health") {
    sendJson(res, 200, { ok: true, service: "aegisguard-backend", stage: "framework-ready" });
    return true;
  }
  if (req.method === "GET" && pathname === "/api/agents") {
    sendJson(res, 200, { items: listAgents() });
    return true;
  }
  if (req.method === "GET" && pathname === "/api/attack-families") {
    sendJson(res, 200, { items: listAttackFamilies() });
    return true;
  }
  if (req.method === "GET" && pathname === "/api/experiment-layers") {
    sendJson(res, 200, { items: listExperimentLayers() });
    return true;
  }
  if (req.method === "GET" && pathname === "/api/experiment-plan") {
    sendJson(res, 200, getExperimentPlan());
    return true;
  }
  if (req.method === "GET" && pathname === "/api/scenarios") {
    sendJson(res, 200, { scenarios: getScenarioTemplates() });
    return true;
  }
  if (req.method === "GET" && pathname === "/api/audit") {
    sendJson(res, 200, { items: listAudit() });
    return true;
  }
  if (req.method === "DELETE" && pathname === "/api/audit") {
    clearAudit();
    sendJson(res, 200, { ok: true });
    return true;
  }
  if (req.method === "POST" && pathname === "/api/issue-token") {
    const body = await collectBody(req);
    sendJson(res, 200, { token: issueRequireToken(body) });
    return true;
  }
  if (req.method === "POST" && pathname === "/api/verify-request") {
    const body = await collectBody(req);
    const verification = verifyRequireToken(body.token, body);
    const gateDecision = decideGate(body, verification);
    const sandboxResult = runMemorySandbox(body.rawResult || "");
    const entry = buildAuditEntry(body, body.token, verification, gateDecision, sandboxResult);
    persistAudit(entry);
    sendJson(res, 200, {
      verification,
      gateDecision,
      sandboxResult,
      auditLogs: entry.logs,
      auditEntry: entry
    });
    return true;
  }
  return false;
}

module.exports = {
  handleApiRequest
};
