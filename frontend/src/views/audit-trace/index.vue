<script setup lang="ts">
import { onMounted } from "vue";
import AttackTimeline from "@/components/audit/AttackTimeline.vue";
import EvidenceCard from "@/components/audit/EvidenceCard.vue";
import StatCard from "@/components/common/StatCard.vue";
import { useAuditStream } from "@/hooks/useAuditStream";

defineOptions({ name: "AuditTraceIndex" });

const { events, attackChains, stats, loadLogs, loadChains, loadStats, loading } =
  useAuditStream();

onMounted(() => {
  Promise.all([loadLogs(), loadChains(), loadStats()]);
});
</script>

<template>
  <div class="audit-trace p-4">
    <h1 class="text-2xl font-bold mb-4">Audit Trail</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="6">
        <StatCard
          title="Total Events"
          :value="stats?.total_events || 0"
          color="var(--aegis-primary)"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="Today Events"
          :value="stats?.today_events || 0"
          color="var(--aegis-info)"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="Attack Chains"
          :value="stats?.attack_chains || 0"
          color="var(--aegis-danger)"
        />
      </el-col>
      <el-col :span="6">
        <StatCard
          title="Avg Duration"
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
            <span class="font-semibold">Attack Chains</span>
          </template>
          <div v-loading="loading" class="space-y-4">
            <AttackTimeline
              v-for="chain in attackChains"
              :key="chain.chain_id"
              :chain="chain"
            />
            <el-empty
              v-if="!attackChains.length && !loading"
              description="No attack chains detected"
            />
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">Decision Distribution</span>
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
                  :percentage="
                    Math.round(
                      (count / (stats.today_events || 1)) * 100
                    )
                  "
                  :stroke-width="6"
                  style="width: 100px"
                />
                <span class="text-xs text-gray-500 w-8 text-right">
                  {{ count }}
                </span>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">Top Agents</span>
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
        <span class="font-semibold">Recent Audit Events</span>
      </template>
      <el-table :data="events" stripe size="small" v-loading="loading">
        <el-table-column prop="timestamp" label="Time" width="180">
          <template #default="{ row }">
            {{ new Date(row.timestamp).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column prop="event_type" label="Type" width="120">
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="
                row.event_type === 'block'
                  ? 'danger'
                  : row.event_type === 'allow'
                    ? 'success'
                    : 'info'
              "
            >
              {{ row.event_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="decision" label="Decision" width="120" />
        <el-table-column prop="risk_score" label="Risk" width="80">
          <template #default="{ row }">
            {{ Math.round(row.risk_score * 100) }}%
          </template>
        </el-table-column>
        <el-table-column prop="agent_id" label="Agent" width="120" />
        <el-table-column prop="tool_name" label="Tool" width="120" />
        <el-table-column
          prop="description"
          label="Description"
          min-width="250"
          show-overflow-tooltip
        />
        <el-table-column prop="status" label="HTTP" width="80">
          <template #default="{ row }">
            <el-tag
              :type="row.status < 400 ? 'success' : 'danger'"
              size="small"
            >
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
