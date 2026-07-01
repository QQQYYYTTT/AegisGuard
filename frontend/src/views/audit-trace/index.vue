<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRoute } from "vue-router";
import AttackChainGraph from "@/components/audit/AttackChainGraph.vue";
import EvidenceCard from "@/components/audit/EvidenceCard.vue";
import StatCard from "@/components/common/StatCard.vue";
import { useAuditStream } from "@/hooks/useAuditStream";
import type { AuditEvent, AttackChain } from "@/api/audit";

defineOptions({ name: "AuditTraceIndex" });

const { events, attackChains, stats, loadLogs, loadChains, loadStats, loading, startPolling, stopPolling } =
  useAuditStream();

const route = useRoute();

const selectedChain = ref<AttackChain | null>(null);
const activeTab = ref("summary");

const selectedEvent = ref<AuditEvent | null>(null);

const demoTraceEvents: AuditEvent[] = [
  {
    id: "demo-trace-1",
    request_id: "demo-chain-openclaw-001",
    timestamp: new Date(Date.now() - 11 * 60_000).toISOString(),
    method: "POST",
    path: "/aegis/gate/evaluate",
    status_code: 200,
    status: 200,
    duration_ms: 44,
    decision: "Allow",
    risk_score: 24,
    risk_level: "low",
    gate_type: "message",
    reason: "OpenClaw 测试流量进入 Message Gate。",
    token_status: "valid",
    auth_mode: "strict",
    unauthorized_allow: false,
    agent_id: "openclaw",
    session_id: "demo-session-openclaw",
    tool_name: "",
    body_hash: "demo-trace-hash-1",
    event_type: "gate",
    description: "策略引擎解析 DPI 测试载荷。"
  },
  {
    id: "demo-trace-2",
    request_id: "demo-chain-openclaw-001",
    timestamp: new Date(Date.now() - 10 * 60_000).toISOString(),
    method: "POST",
    path: "/aegis/gate/evaluate",
    status_code: 403,
    status: 403,
    duration_ms: 68,
    decision: "Block",
    risk_score: 100,
    risk_level: "critical",
    gate_type: "message",
    reason: "DPI 直接提示注入命中系统覆盖规则。",
    token_status: "valid",
    auth_mode: "strict",
    unauthorized_allow: false,
    agent_id: "openclaw",
    session_id: "demo-session-openclaw",
    tool_name: "",
    body_hash: "demo-trace-hash-2",
    event_type: "gate",
    description: "Message Gate 阻断直接提示注入。"
  },
  {
    id: "demo-trace-3",
    request_id: "demo-chain-openclaw-001",
    timestamp: new Date(Date.now() - 9 * 60_000).toISOString(),
    method: "POST",
    path: "/aegis/sandbox/isolate",
    status_code: 200,
    status: 200,
    duration_ms: 52,
    decision: "Block",
    risk_score: 82,
    risk_level: "high",
    gate_type: "return",
    reason: "MP 记忆投毒候选内容被隔离。",
    token_status: "valid",
    auth_mode: "strict",
    unauthorized_allow: false,
    agent_id: "openclaw",
    session_id: "demo-session-openclaw",
    tool_name: "memory.write",
    body_hash: "demo-trace-hash-3",
    event_type: "sandbox",
    description: "沙箱阻止高风险内容进入可信记忆。"
  }
];

const demoAttackChains: AttackChain[] = [
  {
    chain_id: "demo-chain-openclaw-001",
    events: demoTraceEvents,
    start_time: demoTraceEvents[0].timestamp,
    end_time: demoTraceEvents[demoTraceEvents.length - 1].timestamp,
    severity: "critical",
    summary: "OpenClaw DPI + MP 测试链路：提示注入被 Message Gate 阻断，记忆投毒候选进入沙箱隔离。"
  }
];

const displayAttackChains = computed(() =>
  attackChains.value?.length ? attackChains.value : demoAttackChains
);

const chainSummary = computed(() => {
  if (!selectedChain.value) return null;
  const chain = selectedChain.value;
  const totalEvents = chain.events.length;
  const blockedEvents = chain.events.filter(e => e.decision === "Deny" || e.decision === "Block").length;
  const authEvents = chain.events.filter(e => e.event_type === "authorization").length;
  const sandboxEvents = chain.events.filter(e => e.event_type === "sandbox").length;
  const gateEvents = chain.events.filter(e => e.event_type === "gate").length;
  
  const uniqueAgents = [...new Set(chain.events.map(e => e.agent_id).filter(Boolean))];
  const uniqueIPs = [...new Set(chain.events.map(e => (e as any).client_ip).filter(Boolean))];
  const uniqueHosts = [...new Set(chain.events.map(e => (e as any).host).filter(Boolean))];
  
  const startTime = new Date(chain.start_time).getTime();
  const endTime = new Date(chain.end_time).getTime();
  const duration = (!isNaN(startTime) && !isNaN(endTime)) ? endTime - startTime : 0;

  return {
    chainId: chain.chain_id,
    severity: chain.severity,
    summary: chain.summary,
    totalEvents,
    blockedEvents,
    authEvents,
    sandboxEvents,
    gateEvents,
    uniqueAgents,
    uniqueIPs,
    uniqueHosts,
    startTime: chain.start_time,
    endTime: chain.end_time,
    duration
  };
});

const gateInterceptInfo = computed(() => {
  if (!selectedChain.value) return [];
  return selectedChain.value.events
    .filter(e => e.event_type === "gate" && (e.decision === "Deny" || e.decision === "Block"))
    .map(e => ({
      eventId: e.id,
      timestamp: e.timestamp,
      gateType: (e as any).gate_type || "unknown",
      riskScore: e.risk_score,
      riskLevel: (e as any).risk_level || "unknown",
      reason: (e as any).reason || "unknown",
      decision: e.decision
    }));
});

const authVerifyInfo = computed(() => {
  if (!selectedChain.value) return [];
  return selectedChain.value.events
    .filter(e => e.event_type === "authorization")
    .map(e => ({
      eventId: e.id,
      timestamp: e.timestamp,
      authMode: (e as any).auth_mode || "unknown",
      tokenStatus: (e as any).token_status || "unknown",
      decision: e.decision,
      description: e.description
    }));
});

const sandboxIsolationInfo = computed(() => {
  if (!selectedChain.value) return [];
  return selectedChain.value.events
    .filter(e => e.event_type === "sandbox")
    .map(e => ({
      eventId: e.id,
      timestamp: e.timestamp,
      decision: e.decision,
      description: e.description,
      riskScore: e.risk_score
    }));
});

const keyEvidence = computed(() => {
  if (!selectedChain.value) return [];
  return selectedChain.value.events
    .filter(e => e.risk_score >= 70 || e.decision === "Deny" || e.decision === "Block")
    .slice(0, 5);
});

function selectChain(chain: AttackChain) {
  selectedChain.value = chain;
  activeTab.value = "summary";
}

function selectEvent(event: AuditEvent) {
  selectedEvent.value = event;
}

onMounted(() => {
  Promise.all([loadLogs(), loadChains(), loadStats()]).then(() => {
    const chainParam = (route.query.chain || route.query.request_id) as string;
    if (chainParam) {
      const found = displayAttackChains.value.find(
        c => c.chain_id === chainParam || c.events.some(e => e.request_id === chainParam)
      );
      if (found) {
        selectChain(found);
      }
    } else if (!selectedChain.value && displayAttackChains.value.length) {
      selectChain(displayAttackChains.value[0]);
    }
  });
  startPolling(5000);
});

onUnmounted(() => {
  stopPolling();
});
</script>

<template>
  <div class="audit-trace p-4">
    <h1 class="text-2xl font-bold mb-4">攻击路径溯源 / Attack Path Trace</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="6">
        <StatCard title="总事件数 / Total Events" :value="stats?.total_events || 0" color="var(--aegis-primary)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="今日事件 / Today Events" :value="stats?.today_events || 0" color="var(--aegis-info)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="攻击链数 / Attack Chains" :value="stats?.attack_chains || 0" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="平均耗时 / Avg Duration"
          :value="stats?.avg_duration_ms || 0"
          suffix="ms"
          color="var(--aegis-warning)"
        />
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="8">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">攻击链列表 / Attack Chains</span>
          </template>
          <div v-loading="loading" class="space-y-2 max-h-96 overflow-y-auto">
            <div
              v-for="chain in displayAttackChains"
              :key="chain.chain_id"
              class="p-3 border rounded cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
              :class="{ 'border-blue-500 bg-blue-50 dark:bg-blue-900': selectedChain?.chain_id === chain.chain_id }"
              @click="selectChain(chain)"
            >
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium text-sm">{{ chain.chain_id }}</span>
                <el-tag
                  :type="chain.severity === 'critical' ? 'danger' : chain.severity === 'high' ? 'danger' : chain.severity === 'medium' ? 'warning' : 'info'"
                  size="small"
                  effect="dark"
                >
                  {{ chain.severity.toUpperCase() }}
                </el-tag>
              </div>
              <div class="text-xs text-gray-500 truncate">{{ chain.summary }}</div>
              <div class="text-xs text-gray-400 mt-1">
                {{ chain.events.length }} 事件 | {{ new Date(chain.start_time).toLocaleTimeString() }}
              </div>
            </div>
            <el-empty v-if="!displayAttackChains.length && !loading" description="暂未检测到攻击链" />
          </div>
        </el-card>
      </el-col>

      <el-col :span="16">
        <el-card shadow="hover" class="mb-4" v-if="selectedChain">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-semibold">事件详情 / Event Details</span>
              <el-tag size="small" type="info">{{ selectedChain.chain_id }}</el-tag>
            </div>
          </template>

          <el-tabs v-model="activeTab" class="mb-4">
            <el-tab-pane label="事件摘要 / Summary" name="summary" />
            <el-tab-pane label="攻击链路 / Attack Chain" name="chain" />
            <el-tab-pane label="用户/IP/主机 / Entities" name="entities" />
            <el-tab-pane label="关键证据 / Evidence" name="evidence" />
          </el-tabs>

          <div v-show="activeTab === 'summary'">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="攻击链 ID / Chain ID">
                <code class="text-xs">{{ chainSummary?.chainId }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="严重程度 / Severity">
                <el-tag
                  :type="chainSummary?.severity === 'critical' ? 'danger' : chainSummary?.severity === 'high' ? 'danger' : chainSummary?.severity === 'medium' ? 'warning' : 'info'"
                  size="small"
                  effect="dark"
                >
                  {{ chainSummary?.severity?.toUpperCase() }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="事件总数 / Total Events" :span="2">
                {{ chainSummary?.totalEvents }}
              </el-descriptions-item>
              <el-descriptions-item label="拦截事件 / Blocked Events">
                <el-tag type="danger" size="small">{{ chainSummary?.blockedEvents }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="持续时间 / Duration">
                {{ chainSummary?.duration ? `${(chainSummary.duration / 1000).toFixed(2)}s` : '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="时间范围 / Time Range" :span="2">
                {{ new Date(chainSummary?.startTime || '').toLocaleString() }} - {{ new Date(chainSummary?.endTime || '').toLocaleString() }}
              </el-descriptions-item>
              <el-descriptions-item label="摘要 / Summary" :span="2">
                {{ chainSummary?.summary }}
              </el-descriptions-item>
            </el-descriptions>

            <el-divider>闸门拦截统计 / Gate Interception</el-divider>
            <el-row :gutter="16" class="mb-4">
              <el-col :span="8">
                <StatCard title="闸门事件 / Gate Events" :value="chainSummary?.gateEvents || 0" color="var(--aegis-warning)" />
              </el-col>
              <el-col :span="8">
                <StatCard title="授权事件 / Auth Events" :value="chainSummary?.authEvents || 0" color="var(--aegis-primary)" />
              </el-col>
              <el-col :span="8">
                <StatCard title="沙箱事件 / Sandbox Events" :value="chainSummary?.sandboxEvents || 0" color="var(--aegis-info)" />
              </el-col>
            </el-row>
          </div>

          <div v-if="activeTab === 'chain'">
            <AttackChainGraph :chain="selectedChain" />
          </div>

          <div v-show="activeTab === 'entities'">
            <el-row :gutter="16">
              <el-col :span="8">
                <el-card shadow="never">
                  <template #header>
                    <span class="font-semibold">涉及用户 / Users</span>
                  </template>
                  <div v-if="chainSummary?.uniqueAgents.length" class="space-y-2">
                    <el-tag v-for="agent in chainSummary.uniqueAgents" :key="agent" size="small" class="mr-2 mb-2">
                      {{ agent }}
                    </el-tag>
                  </div>
                  <el-empty v-else description="无" :image-size="60" />
                </el-card>
              </el-col>
              <el-col :span="8">
                <el-card shadow="never">
                  <template #header>
                    <span class="font-semibold">涉及 IP / IP Addresses</span>
                  </template>
                  <div v-if="chainSummary?.uniqueIPs.length" class="space-y-2">
                    <el-tag v-for="ip in chainSummary.uniqueIPs" :key="ip" size="small" type="info" class="mr-2 mb-2">
                      {{ ip }}
                    </el-tag>
                  </div>
                  <el-empty v-else description="无" :image-size="60" />
                </el-card>
              </el-col>
              <el-col :span="8">
                <el-card shadow="never">
                  <template #header>
                    <span class="font-semibold">涉及主机 / Hosts</span>
                  </template>
                  <div v-if="chainSummary?.uniqueHosts.length" class="space-y-2">
                    <el-tag v-for="host in chainSummary.uniqueHosts" :key="host" size="small" type="warning" class="mr-2 mb-2">
                      {{ host }}
                    </el-tag>
                  </div>
                  <el-empty v-else description="无" :image-size="60" />
                </el-card>
              </el-col>
            </el-row>
          </div>

          <div v-show="activeTab === 'evidence'">
            <div class="space-y-4">
              <EvidenceCard v-for="evidence in keyEvidence" :key="evidence.id" :evidence="evidence as any" />
              <el-empty v-if="!keyEvidence.length" description="无关键证据" :image-size="80" />
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" v-if="selectedChain">
          <template #header>
            <span class="font-semibold">安全特性 / Security Features</span>
          </template>

          <el-tabs>
            <el-tab-pane label="可信授权校验 / Auth Verification">
              <el-table :data="authVerifyInfo" stripe size="small">
                <el-table-column prop="timestamp" label="时间 / Time" width="180">
                  <template #default="{ row }">
                    {{ new Date(row.timestamp).toLocaleString() }}
                  </template>
                </el-table-column>
                <el-table-column prop="authMode" label="授权模式 / Mode" width="120">
                  <template #default="{ row }">
                    <el-tag size="small" type="info">{{ row.authMode }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="tokenStatus" label="Token 状态 / Status" width="120">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.tokenStatus === 'valid' ? 'success' : 'danger'">{{ row.tokenStatus }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="decision" label="决策 / Decision" width="100">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.decision === 'Allow' ? 'success' : 'danger'">{{ row.decision }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="description" label="描述 / Description" min-width="200" show-overflow-tooltip />
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="闸门拦截位置 / Gate Interception">
              <el-table :data="gateInterceptInfo" stripe size="small">
                <el-table-column prop="timestamp" label="时间 / Time" width="180">
                  <template #default="{ row }">
                    {{ new Date(row.timestamp).toLocaleString() }}
                  </template>
                </el-table-column>
                <el-table-column prop="gateType" label="闸门类型 / Gate Type" width="120">
                  <template #default="{ row }">
                    <el-tag size="small" type="warning">{{ row.gateType }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="riskScore" label="风险分数 / Risk Score" width="110" />
                <el-table-column prop="riskLevel" label="风险等级 / Risk Level" width="110">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.riskLevel === 'high' || row.riskLevel === 'critical' ? 'danger' : row.riskLevel === 'medium' ? 'warning' : 'info'">
                      {{ row.riskLevel }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="decision" label="决策 / Decision" width="100">
                  <template #default="{ row }">
                    <el-tag size="small" type="danger">{{ row.decision }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="reason" label="原因 / Reason" min-width="200" show-overflow-tooltip />
              </el-table>
            </el-tab-pane>

            <el-tab-pane label="记忆沙箱隔离 / Sandbox Isolation">
              <el-table :data="sandboxIsolationInfo" stripe size="small">
                <el-table-column prop="timestamp" label="时间 / Time" width="180">
                  <template #default="{ row }">
                    {{ new Date(row.timestamp).toLocaleString() }}
                  </template>
                </el-table-column>
                <el-table-column prop="riskScore" label="风险分数 / Risk Score" width="110" />
                <el-table-column prop="decision" label="决策 / Decision" width="100">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.decision === 'Allow' ? 'success' : 'danger'">{{ row.decision }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="description" label="描述 / Description" min-width="300" show-overflow-tooltip />
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </el-card>

        <el-empty v-if="!selectedChain" description="请选择一个攻击链查看详情" :image-size="120" />
      </el-col>
    </el-row>
  </div>
</template>
