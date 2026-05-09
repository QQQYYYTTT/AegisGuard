<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import StatCard from "@/components/common/StatCard.vue";
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import RealTimeTag from "@/components/common/RealTimeTag.vue";
import { useGateDecision } from "@/hooks/useGateDecision";
import { useAuditStream } from "@/hooks/useAuditStream";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";

defineOptions({ name: "DashboardIndex" });

const { overview, decisions, loadOverview, startPolling: startGatePolling, stopPolling: stopGatePolling } = useGateDecision();
const { stats, loadStats, startPolling: startAuditPolling, stopPolling: stopAuditPolling } = useAuditStream();
const { sm2Status, sm3Status, sm4Status, activeTokens } = useCryptoStatus();

const runningAgents = ref(5);

onMounted(async () => {
  await Promise.all([loadOverview(), loadStats()]);
  startGatePolling(5000);
  startAuditPolling(5000);
});

onUnmounted(() => {
  stopGatePolling();
  stopAuditPolling();
});

const todayBlocks = ref(113);
const avgLatency = ref(85);
const todayEvents = ref(1523);
</script>

<template>
  <div class="dashboard p-4">
    <div class="mb-6">
      <h1 class="text-2xl font-bold mb-2">系统总览 / AegisGuard System Overview</h1>
      <div class="flex gap-2">
        <RealTimeTag label="SM2" :active="sm2Status" />
        <RealTimeTag label="SM3" :active="sm3Status" />
        <RealTimeTag label="SM4" :active="sm4Status" />
        <RealTimeTag label="Action Gate" :active="true" />
        <RealTimeTag label="Sandbox" :active="true" />
      </div>
    </div>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="6">
        <StatCard
          title="运行 Agent / Running Agents"
          :value="runningAgents"
          icon="ep:user-filled"
          color="var(--aegis-primary)"
          trend="up"
          trend-value="今日 +2"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="今日拦截 / Today Blocks"
          :value="todayBlocks"
          icon="ep:warning-filled"
          color="var(--aegis-danger)"
          trend="up"
          trend-value="较昨日 +15"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="平均时延 / Avg Latency"
          :value="avgLatency"
          suffix="ms"
          icon="ep:timer"
          color="var(--aegis-warning)"
          trend="down"
          trend-value="-12ms"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="今日事件 / Today Events"
          :value="todayEvents"
          icon="ep:document"
          color="var(--aegis-info)"
        />
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="16">
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">最近决策 / Recent Decisions</span>
          </template>
          <el-table :data="decisions" stripe size="small">
            <el-table-column prop="timestamp" label="时间 / Time" width="180">
              <template #default="{ row }">
                {{ new Date(row.timestamp).toLocaleTimeString() }}
              </template>
            </el-table-column>
            <el-table-column prop="gate_type" label="闸门 / Gate" width="100">
              <template #default="{ row }">
                <el-tag size="small" type="info">{{ row.gate_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="decision" label="决策 / Decision" width="120">
              <template #default="{ row }">
                <DecisionBadge :decision="row.decision" />
              </template>
            </el-table-column>
            <el-table-column prop="risk_level" label="风险 / Risk" width="100">
              <template #default="{ row }">
                <RiskBadge :level="row.risk_level" />
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="原因 / Reason" min-width="200" show-overflow-tooltip />
            <el-table-column prop="agent_id" label="Agent" width="120" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">闸门状态 / Gate Status</span>
          </template>
          <div v-if="overview" class="space-y-3">
            <div class="flex items-center justify-between p-2 rounded bg-gray-50 dark:bg-gray-800">
              <span class="text-sm">消息闸门 / Message Gate</span>
              <div class="flex items-center gap-2">
                <el-tag type="success" size="small">{{ overview.message_gate.status }}</el-tag>
                <span class="text-xs text-gray-500">
                  {{ overview.message_gate.block_count }} 次拦截
                </span>
              </div>
            </div>
            <div class="flex items-center justify-between p-2 rounded bg-gray-50 dark:bg-gray-800">
              <span class="text-sm">动作闸门 / Action Gate</span>
              <div class="flex items-center gap-2">
                <el-tag type="success" size="small">{{ overview.action_gate.status }}</el-tag>
                <span class="text-xs text-gray-500">
                  {{ overview.action_gate.block_count }} 次拦截
                </span>
              </div>
            </div>
            <div class="flex items-center justify-between p-2 rounded bg-gray-50 dark:bg-gray-800">
              <span class="text-sm">返回闸门 / Return Gate</span>
              <div class="flex items-center gap-2">
                <el-tag type="success" size="small">{{ overview.return_gate.status }}</el-tag>
                <span class="text-xs text-gray-500">
                  {{ overview.return_gate.block_count }} 次拦截
                </span>
              </div>
            </div>
          </div>
        </el-card>
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">活跃令牌 / Active Tokens</span>
          </template>
          <div class="text-center">
            <div class="text-4xl font-bold text-blue-500">{{ activeTokens }}</div>
            <div class="text-sm text-gray-500 mt-1">当前活跃的 SM2 令牌</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>
