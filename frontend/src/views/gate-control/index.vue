<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import StatCard from "@/components/common/StatCard.vue";
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import DisposalFlowGraph from "@/components/audit/DisposalFlowGraph.vue";
import { useGateStoreHook } from "@/store/modules/gate";
import { useAuditStoreHook } from "@/store/modules/audit";
import type { GateDecision } from "@/api/gate";
import type { AuditEvent } from "@/api/audit";

defineOptions({ name: "AutoDisposalCenter" });

const gateStore = useGateStoreHook();
const auditStore = useAuditStoreHook();
const router = useRouter();

const activeTab = ref("overview");
const timeRange = ref("24h");
const selectedGateType = ref("all");

const gateDecisions = computed(() => gateStore.decisions);
const auditEvents = computed(() => auditStore.events);

const disposalStats = computed(() => {
  const decisions = gateDecisions.value || [];
  const events = auditEvents.value || [];
  
  const totalIntercepts = decisions.filter(d => d.decision === "Block" || d.decision === "Deny").length;
  const totalAllowed = decisions.filter(d => d.decision === "Allow").length;
  const totalDecisions = decisions.length;
  const successRate = totalDecisions > 0 ? Math.round((totalIntercepts / totalDecisions) * 100) : 0;
  
  const avgDuration = events.length > 0
    ? Math.round(events.reduce((sum, e) => sum + ((e as any).duration_ms || 0), 0) / events.length)
    : 0;
  
  const messageGateBlocks = decisions.filter(d => d.gate_type === "message" && (d.decision === "Block" || d.decision === "Deny")).length;
  const actionGateBlocks = decisions.filter(d => d.gate_type === "action" && (d.decision === "Block" || d.decision === "Deny")).length;
  const returnGateBlocks = decisions.filter(d => d.gate_type === "return" && (d.decision === "Block" || d.decision === "Deny")).length;
  
  const authDenials = events.filter(e => e.event_type === "authorization" && (e.decision === "Deny" || e.decision === "Block")).length;
  const sandboxIsolations = events.filter(e => e.event_type === "sandbox").length;
  
  const policyHits: Record<string, number> = {};
  decisions.forEach(d => {
    d.matched_rules.forEach(rule => {
      policyHits[rule] = (policyHits[rule] || 0) + 1;
    });
  });
  
  const topPolicies = Object.entries(policyHits)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 10)
    .map(([name, count]) => ({ name, count }));
  
  return {
    totalIntercepts,
    totalAllowed,
    totalDecisions,
    successRate,
    avgDuration,
    messageGateBlocks,
    actionGateBlocks,
    returnGateBlocks,
    authDenials,
    sandboxIsolations,
    topPolicies
  };
});

const interceptionTrend = computed(() => {
  const decisions = gateDecisions.value || [];
  const hourlyData: Record<string, { allow: number; block: number }> = {};
  
  decisions.forEach(d => {
    const hour = new Date(d.timestamp).getHours();
    const key = `${hour}:00`;
    if (!hourlyData[key]) {
      hourlyData[key] = { allow: 0, block: 0 };
    }
    if (d.decision === "Block" || d.decision === "Deny") {
      hourlyData[key].block++;
    } else {
      hourlyData[key].allow++;
    }
  });
  
  return Object.entries(hourlyData)
    .sort((a, b) => parseInt(a[0]) - parseInt(b[0]))
    .map(([hour, data]) => ({ hour, ...data }));
});

const filteredDecisions = computed(() => {
  let decisions = gateDecisions.value || [];
  if (selectedGateType.value !== "all") {
    decisions = decisions.filter(d => d.gate_type === selectedGateType.value);
  }
  return decisions.slice(0, 50);
});

const authDenialRecords = computed(() => {
  const list = auditEvents.value || [];
  return list
    .filter(e => e.event_type === "authorization" && (e.decision === "Deny" || e.decision === "Block"))
    .slice(0, 20)
    .map(e => ({
      timestamp: e.timestamp,
      agentId: e.agent_id,
      sessionId: e.session_id,
      decision: e.decision,
      authMode: (e as any).auth_mode,
      tokenStatus: (e as any).token_status,
      reason: (e as any).reason || e.description
    }));
});

const sandboxIsolationRecords = computed(() => {
  const list = auditEvents.value || [];
  return list
    .filter(e => e.event_type === "sandbox")
    .slice(0, 20)
    .map(e => ({
      timestamp: e.timestamp,
      agentId: e.agent_id,
      sessionId: e.session_id,
      decision: e.decision,
      riskScore: e.risk_score || 0,
      riskLevel: (e as any).risk_level || "low",
      description: e.description
    }));
});

let refreshTimer: ReturnType<typeof setInterval> | null = null;

async function refreshData() {
  try {
    await Promise.all([
      gateStore.fetchDecisions(),
      auditStore.fetchLogs()
    ]);
  } catch (e) {
    console.warn('gate-control refresh failed:', e);
  }
}

onMounted(() => {
  refreshData();
  refreshTimer = setInterval(refreshData, 10000);
});

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
});

function viewAlertDetail(alert: any) {
  // TODO: 实现告警详情查看
  console.log('View alert detail:', alert);
}
</script>

<template>
  <div class="auto-disposal-center p-4">
    <h1 class="text-2xl font-bold mb-4">自动处置中心 / Auto Disposal Center</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="6">
        <StatCard title="自动拦截次数 / Total Intercepts" :value="disposalStats.totalIntercepts" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="处置成功率 / Success Rate" :value="disposalStats.successRate" suffix="%" color="var(--aegis-success)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="平均处置时延 / Avg Duration" :value="disposalStats.avgDuration" suffix="ms" color="var(--aegis-warning)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="总处置次数 / Total Decisions" :value="disposalStats.totalDecisions" color="var(--aegis-primary)" />
      </el-col>
    </el-row>

    <el-card shadow="hover" class="mb-4">
      <template #header>
        <div class="flex items-center justify-between">
          <span class="font-semibold">处置效果 / Disposal Effectiveness</span>
          <el-radio-group v-model="activeTab" size="small">
            <el-radio-button label="overview">总览</el-radio-button>
            <el-radio-button label="gates">闸门拦截</el-radio-button>
            <el-radio-button label="auth">授权拒绝</el-radio-button>
            <el-radio-button label="sandbox">沙箱隔离</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <div v-show="activeTab === 'overview'">
        <el-card shadow="never" class="mb-4">
          <template #header>
            <span class="font-semibold text-sm">处置流程拓扑 / Disposal Flow Topology</span>
          </template>
          <DisposalFlowGraph :stats="disposalStats" />
        </el-card>

        <el-row :gutter="16" class="mb-4">
          <el-col :span="8">
            <el-card shadow="never">
              <template #header>
                <span class="font-semibold text-sm">三级闸门拦截效果 / Gate Interception</span>
              </template>
              <el-descriptions :column="1" border size="small">
                <el-descriptions-item label="Message Gate">
                  <div class="flex items-center justify-between">
                    <span>拦截数</span>
                    <el-tag type="danger" size="small">{{ disposalStats.messageGateBlocks }}</el-tag>
                  </div>
                </el-descriptions-item>
                <el-descriptions-item label="Action Gate">
                  <div class="flex items-center justify-between">
                    <span>拦截数</span>
                    <el-tag type="danger" size="small">{{ disposalStats.actionGateBlocks }}</el-tag>
                  </div>
                </el-descriptions-item>
                <el-descriptions-item label="Return Gate">
                  <div class="flex items-center justify-between">
                    <span>拦截数</span>
                    <el-tag type="danger" size="small">{{ disposalStats.returnGateBlocks }}</el-tag>
                  </div>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card shadow="never">
              <template #header>
                <span class="font-semibold text-sm">安全特性拦截 / Security Features</span>
              </template>
              <el-descriptions :column="1" border size="small">
                <el-descriptions-item label="可信授权拒绝">
                  <div class="flex items-center justify-between">
                    <span>拒绝数</span>
                    <el-tag type="danger" size="small">{{ disposalStats.authDenials }}</el-tag>
                  </div>
                </el-descriptions-item>
                <el-descriptions-item label="记忆沙箱隔离">
                  <div class="flex items-center justify-between">
                    <span>隔离数</span>
                    <el-tag type="info" size="small">{{ disposalStats.sandboxIsolations }}</el-tag>
                  </div>
                </el-descriptions-item>
                <el-descriptions-item label="总拦截数">
                  <div class="flex items-center justify-between">
                    <span>拦截数</span>
                    <el-tag type="danger" size="small">{{ disposalStats.totalIntercepts }}</el-tag>
                  </div>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card shadow="never">
              <template #header>
                <span class="font-semibold text-sm">策略命中 TOP10 / Top Policies</span>
              </template>
              <div class="space-y-2 max-h-64 overflow-y-auto">
                <div
                  v-for="(policy, index) in disposalStats.topPolicies"
                  :key="policy.name"
                  class="flex items-center justify-between p-2 border rounded"
                >
                  <div class="flex items-center gap-2">
                    <span class="text-xs text-gray-400 w-6">{{ index + 1 }}</span>
                    <span class="text-sm">{{ policy.name }}</span>
                  </div>
                  <el-tag size="small" type="warning">{{ policy.count }}</el-tag>
                </div>
                <el-empty v-if="!disposalStats.topPolicies.length" description="暂无数据" :image-size="60" />
              </div>
            </el-card>
          </el-col>
        </el-row>

        <el-card shadow="never">
          <template #header>
            <span class="font-semibold text-sm">拦截趋势 / Interception Trend</span>
          </template>
          <div v-if="interceptionTrend.length" class="space-y-2">
            <div
              v-for="item in interceptionTrend"
              :key="item.hour"
              class="flex items-center gap-2"
            >
              <span class="text-xs text-gray-500 w-12">{{ item.hour }}</span>
              <div class="flex-1 flex gap-1">
                <div
                  class="h-4 bg-green-500 rounded"
                  :style="{ width: `${(item.allow / Math.max(item.allow, item.block)) * 100}%` }"
                />
                <div
                  class="h-4 bg-red-500 rounded"
                  :style="{ width: `${(item.block / Math.max(item.allow, item.block)) * 100}%` }"
                />
              </div>
              <span class="text-xs text-gray-500 w-20 text-right">
                允许: {{ item.allow }} | 拦截: {{ item.block }}
              </span>
            </div>
          </div>
          <el-empty v-else description="暂无趋势数据" :image-size="80" />
        </el-card>
      </div>

      <div v-show="activeTab === 'gates'">
        <div class="mb-3 flex items-center gap-2">
          <span class="text-sm">闸门类型：</span>
          <el-radio-group v-model="selectedGateType" size="small">
            <el-radio-button label="all">全部</el-radio-button>
            <el-radio-button label="message">Message Gate</el-radio-button>
            <el-radio-button label="action">Action Gate</el-radio-button>
            <el-radio-button label="return">Return Gate</el-radio-button>
          </el-radio-group>
        </div>

        <el-table :data="filteredDecisions" stripe size="small" max-height="600" @row-click="(row) => router.push({ path: '/audit-trace', query: { chain: row.request_id } })">
          <el-table-column prop="timestamp" label="时间 / Time" width="180">
            <template #default="{ row }">
              {{ new Date(row.timestamp).toLocaleString() }}
            </template>
          </el-table-column>
          <el-table-column prop="gate_type" label="闸门类型 / Gate" width="120">
            <template #default="{ row }">
              <el-tag size="small" type="warning">{{ row.gate_type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="decision" label="决策 / Decision" width="100">
            <template #default="{ row }">
              <DecisionBadge :decision="row.decision" />
            </template>
          </el-table-column>
          <el-table-column prop="risk_score" label="风险分 / Risk" width="90" />
          <el-table-column prop="risk_level" label="风险等级 / Level" width="100">
            <template #default="{ row }">
              <RiskBadge :level="row.risk_level" />
            </template>
          </el-table-column>
          <el-table-column prop="tool_name" label="工具 / Tool" width="120" />
          <el-table-column prop="agent_id" label="Agent" width="120" />
          <el-table-column prop="reason" label="原因 / Reason" min-width="200" show-overflow-tooltip />
        </el-table>
      </div>

      <div v-show="activeTab === 'auth'">
        <el-alert
          title="可信授权拒绝记录"
          description="展示所有因授权校验失败而被拒绝的请求记录"
          type="warning"
          :closable="false"
          class="mb-4"
        />
        <el-table :data="authDenialRecords" stripe size="small" max-height="600">
          <el-table-column prop="timestamp" label="时间 / Time" width="180">
            <template #default="{ row }">
              {{ new Date(row.timestamp).toLocaleString() }}
            </template>
          </el-table-column>
          <el-table-column prop="agentId" label="Agent" width="120" />
          <el-table-column prop="sessionId" label="会话 ID" width="120" />
          <el-table-column prop="authMode" label="授权模式 / Mode" width="100">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.authMode || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="tokenStatus" label="Token 状态" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="row.tokenStatus === 'valid' ? 'success' : 'danger'">{{ row.tokenStatus || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="decision" label="决策 / Decision" width="100">
            <template #default="{ row }">
              <el-tag size="small" type="danger">{{ row.decision }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="拒绝原因 / Reason" min-width="200" show-overflow-tooltip />
        </el-table>
      </div>

      <div v-show="activeTab === 'sandbox'">
        <el-alert
          title="记忆沙箱隔离记录"
          description="展示所有被隔离到记忆沙箱的会话和事件"
          type="info"
          :closable="false"
          class="mb-4"
        />
        <el-table :data="sandboxIsolationRecords" stripe size="small" max-height="600">
          <el-table-column prop="timestamp" label="时间 / Time" width="180">
            <template #default="{ row }">
              {{ new Date(row.timestamp).toLocaleString() }}
            </template>
          </el-table-column>
          <el-table-column prop="agentId" label="Agent" width="120" />
          <el-table-column prop="sessionId" label="会话 ID" width="120" />
          <el-table-column prop="riskScore" label="风险分 / Risk" width="90" />
          <el-table-column prop="riskLevel" label="风险等级 / Level" width="100">
            <template #default="{ row }">
              <RiskBadge :level="row.riskLevel" />
            </template>
          </el-table-column>
          <el-table-column prop="decision" label="决策 / Decision" width="100">
            <template #default="{ row }">
              <DecisionBadge :decision="row.decision" />
            </template>
          </el-table-column>
          <el-table-column prop="description" label="描述 / Description" min-width="200" show-overflow-tooltip />
        </el-table>
      </div>
    </el-card>
  </div>
</template>
