const { readAuditStore, writeAuditStore } = require("../data/auditStore");

function buildAuditEntry(body, token, verification, gateDecision, sandboxResult) {
  return {
    id: `audit-${Date.now()}`,
    created_at: new Date().toISOString(),
    scenario_id: body.scenarioId || "custom",
    user_goal: body.userGoal,
    tool_name: body.toolName,
    requested_scope: body.requestedScope,
    decision: gateDecision.action,
    stage: gateDecision.stage,
    reason: gateDecision.reason,
    verification,
    token_preview: {
      agent_id: token.agent_id,
      session_id: token.session_id,
      nonce: token.nonce
    },
    sandbox: sandboxResult,
    logs: [
      { step: "Policy Center", text: "已签发 RequireToken，并写入会话绑定信息。" },
      { step: "RequireShield", text: verification.allPassed ? "授权校验通过。" : "授权校验存在异常。" },
      { step: gateDecision.stage, text: `输出 ${gateDecision.action}。${gateDecision.reason}` },
      { step: "Memory Sandbox", text: `已生成安全摘要，指纹 ${sandboxResult.fingerprint_sm3}。` }
    ]
  };
}

function persistAudit(entry) {
  const store = readAuditStore();
  store.unshift(entry);
  writeAuditStore(store.slice(0, 120));
}

function listAudit() {
  return readAuditStore();
}

function clearAudit() {
  writeAuditStore([]);
}

module.exports = {
  buildAuditEntry,
  persistAudit,
  listAudit,
  clearAudit
};
