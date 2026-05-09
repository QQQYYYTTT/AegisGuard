<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import AttackTimeline from "@/components/audit/AttackTimeline.vue";
import StatCard from "@/components/common/StatCard.vue";
import { useAuditStream } from "@/hooks/useAuditStream";

defineOptions({ name: "AuditTraceIndex" });

const { events, attackChains, stats, loadLogs, loadChains, loadStats, loading, startPolling, stopPolling } =
  useAuditStream();

onMounted(() => {
  Promise.all([loadLogs(), loadChains(), loadStats()]);
  startPolling(5000);
});

onUnmounted(() => {
  stopPolling();
});
</script>

<template>
  <div class="audit-trace p-4">
    <h1 class="text-2xl font-bold mb-4">审计追踪 / Audit Trail</h1>

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
      <el-col :span="16">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">攻击链 / Attack Chains</span>
          </template>
          <div v-loading="loading" class="space-y-4">
            <AttackTimeline v-for="chain in attackChains" :key="chain.chain_id" :chain="chain" />
            <el-empty v-if="!attackChains.length && !loading" description="暂未检测到攻击链" />
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">决策分布 / Decision Distribution</span>
          </template>
          <div v-if="stats?.decision_distribution" class="space-y-2">
            <div
              v-for="(count, decision) in stats.decision_distribution"
              :key="decision"
              class="flex items-center justify-between"
            >
              <span class="text-sm">{{ decision }}</span>
              <div class="flex items-center gap-2">
                <el-progress
                  :percentage="Math.round((count / (stats.today_events || 1)) * 100)"
                  :stroke-width="6"
                  style="width: 100px"
                />
                <span class="text-xs text-gray-500 w-8 text-right">{{ count }}</span>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">高频 Agent / Top Agents</span>
          </template>
          <div v-if="stats?.top_agents" class="space-y-2">
            <div
              v-for="agent in stats.top_agents"
              :key="agent.agent_id"
              class="flex items-center justify-between p-1"
            >
              <span class="text-sm">{{ agent.agent_id }}</span>
              <el-tag size="small">{{ agent.count }}</el-tag>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover" class="mt-4">
      <template #header>
        <span class="font-semibold">最近审计事件 / Recent Audit Events</span>
      </template>
      <el-table :data="events" stripe size="small" v-loading="loading">
        <el-table-column prop="timestamp" label="时间 / Time" width="180">
          <template #default="{ row }">
            {{ new Date(row.timestamp).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column prop="gate_type" label="闸门 / Gate" width="110">
          <template #default="{ row }">
            <el-tag size="small" type="info">{{ row.gate_type || "-" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="decision" label="决策 / Decision" width="120" />
        <el-table-column prop="risk_level" label="级别 / Level" width="100">
          <template #default="{ row }">
            {{ row.risk_level || "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="risk_score" label="分数 / Score" width="90" />
        <el-table-column prop="auth_mode" label="模式 / Mode" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.auth_mode" size="small" type="info">{{ row.auth_mode }}</el-tag>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="token_status" label="Token" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.token_status" size="small" type="warning">{{ row.token_status }}</el-tag>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="unauthorized_allow" label="未授权 / Unauthorized" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.unauthorized_allow" size="small" type="danger">allowed</el-tag>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="tool_name" label="工具 / Tool" width="120" />
        <el-table-column prop="reason" label="原因 / Reason" min-width="260" show-overflow-tooltip />
        <el-table-column prop="status_code" label="状态 / HTTP" width="90">
          <template #default="{ row }">
            <el-tag :type="(row.status_code || row.status) < 400 ? 'success' : 'danger'" size="small">
              {{ row.status_code || row.status }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
