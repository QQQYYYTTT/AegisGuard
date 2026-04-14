const api = {
  async get(url) {
    const res = await fetch(url);
    return res.json();
  },
  async post(url, payload) {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
    return res.json();
  }
};

const refs = {
  planPanel: document.getElementById("planPanel"),
  agentsPanel: document.getElementById("agentsPanel"),
  attacksPanel: document.getElementById("attacksPanel"),
  layersPanel: document.getElementById("layersPanel"),
  simUserGoal: document.getElementById("simUserGoal"),
  simAgentId: document.getElementById("simAgentId"),
  simSessionId: document.getElementById("simSessionId"),
  simToolName: document.getElementById("simToolName"),
  simScope: document.getElementById("simScope"),
  simRequestedScope: document.getElementById("simRequestedScope"),
  simRawResult: document.getElementById("simRawResult"),
  tokenOutput: document.getElementById("tokenOutput"),
  verificationOutput: document.getElementById("verificationOutput"),
  auditOutput: document.getElementById("auditOutput"),
  issueTokenButton: document.getElementById("issueTokenButton"),
  verifyRequestButton: document.getElementById("verifyRequestButton"),
  loadPromptScenario: document.getElementById("loadPromptScenario"),
  loadReplayScenario: document.getElementById("loadReplayScenario"),
  loadPoisonScenario: document.getElementById("loadPoisonScenario")
};

let scenarioTemplates = {};
let issuedToken = null;

function renderPlan(plan) {
  refs.planPanel.innerHTML = `
    <div class="info-row"><span>当前阶段</span><strong>${plan.stage}</strong></div>
    <div class="info-row"><span>重点任务</span><strong>${plan.focus}</strong></div>
    <div class="info-row"><span>攻击家族</span><strong>${plan.dimensions.attackFamilies} 类</strong></div>
    <div class="info-row"><span>单 Agent 建议实验量</span><strong>${plan.suggestedRuns.perAgent} 次</strong></div>
  `;
}

function renderAgents(items) {
  refs.agentsPanel.innerHTML = items.map((item) => `
    <article class="mini-card">
      <div class="card-title-row">
        <strong>${item.name}</strong>
        <span class="tag">${item.status}</span>
      </div>
      <p>${item.category}</p>
      <p>原生安全：${item.nativeSecurity.join(" / ")}</p>
      <p>测试重点：${item.testFocus.join(" / ")}</p>
    </article>
  `).join("");
}

function renderAttackFamilies(items) {
  refs.attacksPanel.innerHTML = items.map((item) => `
    <article class="mini-card">
      <strong>${item.name}</strong>
      <p>攻击目标：${item.target}</p>
      <p>关键拦截阶段：${item.gate}</p>
      <p>变体：${item.variants.join(" / ")}</p>
    </article>
  `).join("");
}

function renderLayers(items) {
  refs.layersPanel.innerHTML = items.map((item) => `
    <article class="mini-card">
      <strong>${item.name}</strong>
      <p>${item.description}</p>
      <p>目标：${item.goal}</p>
    </article>
  `).join("");
}

function fillScenario(id) {
  const scenario = scenarioTemplates[id];
  if (!scenario) return;
  refs.simUserGoal.value = scenario.userGoal;
  refs.simAgentId.value = scenario.agentId;
  refs.simSessionId.value = scenario.sessionId;
  refs.simToolName.value = scenario.toolName;
  refs.simScope.value = scenario.scope;
  refs.simRequestedScope.value = scenario.requestedScope;
  refs.simRawResult.value = scenario.rawResult;
  issuedToken = null;
  refs.tokenOutput.textContent = "等待签发";
  refs.verificationOutput.textContent = "等待执行";
  refs.auditOutput.textContent = "等待生成";
}

function collectSimulationForm() {
  return {
    userGoal: refs.simUserGoal.value,
    agentId: refs.simAgentId.value,
    sessionId: refs.simSessionId.value,
    toolName: refs.simToolName.value,
    scope: refs.simScope.value,
    requestedScope: refs.simRequestedScope.value,
    rawResult: refs.simRawResult.value
  };
}

async function bootstrap() {
  const [plan, agents, attacks, layers, scenarios] = await Promise.all([
    api.get("/api/experiment-plan"),
    api.get("/api/agents"),
    api.get("/api/attack-families"),
    api.get("/api/experiment-layers"),
    api.get("/api/scenarios")
  ]);

  scenarioTemplates = scenarios.scenarios || {};
  renderPlan(plan);
  renderAgents(agents.items || []);
  renderAttackFamilies(attacks.items || []);
  renderLayers(layers.items || []);
  fillScenario("prompt-injection");

  refs.loadPromptScenario.addEventListener("click", () => fillScenario("prompt-injection"));
  refs.loadReplayScenario.addEventListener("click", () => fillScenario("replay-attack"));
  refs.loadPoisonScenario.addEventListener("click", () => fillScenario("result-poisoning"));

  refs.issueTokenButton.addEventListener("click", async () => {
    issuedToken = (await api.post("/api/issue-token", collectSimulationForm())).token;
    refs.tokenOutput.textContent = JSON.stringify(issuedToken, null, 2);
  });

  refs.verifyRequestButton.addEventListener("click", async () => {
    if (!issuedToken) {
      refs.verificationOutput.textContent = "请先签发 RequireToken。";
      return;
    }
    const result = await api.post("/api/verify-request", {
      ...collectSimulationForm(),
      token: issuedToken
    });
    refs.verificationOutput.textContent = JSON.stringify({
      verification: result.verification,
      gateDecision: result.gateDecision,
      sandboxResult: result.sandboxResult
    }, null, 2);
    refs.auditOutput.textContent = JSON.stringify(result.auditEntry, null, 2);
  });
}

bootstrap().catch((error) => {
  refs.planPanel.innerHTML = `<div class="mini-card"><strong>初始化失败</strong><p>${error.message}</p></div>`;
});
