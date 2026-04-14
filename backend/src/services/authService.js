const { simpleHash, signPayload } = require("../lib/crypto");

function issueRequireToken(body) {
  const payload = {
    tool_name: body.toolName,
    scope: body.scope,
    agent_id: body.agentId,
    session_id: body.sessionId,
    task_id: body.taskId || `task-${Date.now().toString().slice(-6)}`,
    expires_at: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
    nonce: `nonce-${simpleHash(`${body.agentId}:${Date.now()}`)}`,
    risk_level: "medium",
    schema_hash_sm3: simpleHash(`${body.toolName}:${body.scope}`)
  };

  return {
    ...payload,
    sm2_signature: signPayload(payload)
  };
}

function verifyRequireToken(token, body) {
  const signature = signPayload({
    tool_name: token.tool_name,
    scope: token.scope,
    agent_id: token.agent_id,
    session_id: token.session_id,
    task_id: token.task_id,
    expires_at: token.expires_at,
    nonce: token.nonce,
    risk_level: token.risk_level,
    schema_hash_sm3: token.schema_hash_sm3
  });

  const checks = {
    signatureValid: token.sm2_signature === signature,
    notExpired: new Date(token.expires_at).getTime() > Date.now(),
    sessionMatch: token.session_id === body.sessionId,
    scopeAllowed: body.requestedScope === token.scope || String(body.requestedScope).startsWith(token.scope),
    agentMatch: token.agent_id === body.agentId,
    schemaMatch: token.schema_hash_sm3 === simpleHash(`${token.tool_name}:${token.scope}`)
  };

  return {
    ...checks,
    allPassed: Object.values(checks).every(Boolean)
  };
}

module.exports = {
  issueRequireToken,
  verifyRequireToken
};
