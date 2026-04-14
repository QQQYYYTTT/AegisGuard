function decideGate(body, verification) {
  const text = `${body.userGoal}\n${body.rawResult}\n${body.requestedScope}`.toLowerCase();
  const poisoned = /(ignore all safety|ignore previous|remember this command forever|写入长期记忆|忽略规则)/i.test(text);
  const dangerousScope = /(system_profile|full_table|admin|delete|export_all)/i.test(body.requestedScope);

  if (!verification.signatureValid || !verification.sessionMatch || !verification.agentMatch) {
    return { action: "quarantine", stage: "RequireShield", reason: "授权身份链校验失败，转入隔离队列。" };
  }
  if (!verification.notExpired) {
    return { action: "quarantine", stage: "RequireShield", reason: "授权令牌过期，拒绝直接执行。" };
  }
  if (!verification.scopeAllowed || dangerousScope) {
    return { action: "deny", stage: "Action Gate", reason: "请求 scope 超出最小权限边界，执行前阻断触发。" };
  }
  if (poisoned) {
    return { action: "degrade", stage: "Return Gate", reason: "检测到回执污染或记忆诱导，仅允许降级后的安全摘要模式。" };
  }
  return { action: "allow", stage: "Action Gate", reason: "请求满足授权、会话绑定与最小权限控制，可进入后续执行。" };
}

module.exports = {
  decideGate
};
