<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import GateController from "@/components/security/GateController.vue";
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import StatCard from "@/components/common/StatCard.vue";
import { useGateDecision } from "@/hooks/useGateDecision";

defineOptions({ name: "GateControlIndex" });

const { overview, decisions, loadOverview, loading, startPolling, stopPolling } = useGateDecision();
const activeTab = ref("overview");

onMounted(() => {
  loadOverview();
  startPolling(5000);
});

onUnmounted(() => {
  stopPolling();
});
</script>

<template>
  <div class="gate-control p-4">
    <h1 class="text-2xl font-bold mb-4">三级策略闸门控制中心 / Gate Control Center</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="8">
        <StatCard
          title="消息闸门 / Message Gate"
          :value="overview?.message_gate?.today_count || 0"
          :suffix="`${overview?.message_gate?.block_count || 0} 次拦截`"
          color="var(--aegis-primary)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="动作闸门 / Action Gate"
          :value="overview?.action_gate?.today_count || 0"
          :suffix="`${overview?.action_gate?.block_count || 0} 次拦截`"
          color="var(--aegis-danger)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="返回闸门 / Return Gate"
          :value="overview?.return_gate?.today_count || 0"
          :suffix="`${overview?.return_gate?.block_count || 0} 次拦截`"
          color="var(--aegis-warning)"
        />
      </el-col>
    </el-row>

    <el-tabs v-model="activeTab" class="mb-4">
      <el-tab-pane label="总览 / Overview" name="overview">
        <el-row :gutter="16">
          <el-col :span="16">
            <el-card shadow="hover">
              <template #header>
                <span class="font-semibold">决策历史 / Decision History</span>
              </template>
              <el-table :data="decisions" stripe size="small" v-loading="loading">
                <el-table-column prop="timestamp" label="时间 / Time" width="160">
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
                <el-table-column prop="risk_score" label="分数 / Score" width="100">
                  <template #default="{ row }">
                    {{ row.risk_score }}%
                  </template>
                </el-table-column>
                <el-table-column prop="reason" label="原因 / Reason" min-width="200" show-overflow-tooltip />
                <el-table-column prop="agent_id" label="Agent" width="120" />
              </el-table>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card shadow="hover">
              <template #header>
                <span class="font-semibold">风险分布 / Risk Score Distribution</span>
              </template>
              <div class="space-y-3">
                <div v-for="d in decisions" :key="d.request_id" class="flex items-center gap-2">
                  <span class="text-xs w-16">{{ d.gate_type }}</span>
                  <el-progress
                    :percentage="d.risk_score"
                    :stroke-width="8"
                    :color="
                      d.risk_score >= 80
                        ? '#ff4d4f'
                        : d.risk_score >= 60
                          ? '#f56c6c'
                          : d.risk_score >= 40
                            ? '#e6a23c'
                            : '#67c23a'
                    "
                  />
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane label="消息闸门 / Message Gate" name="message">
        <el-card shadow="hover">
          <GateController
            :decisions="decisions.filter(d => d.gate_type === 'message')"
          />
          <el-empty
            v-if="!decisions.filter(d => d.gate_type === 'message').length"
            description="暂无消息闸门决策记录"
          />
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="动作闸门 / Action Gate" name="action">
        <el-card shadow="hover">
          <GateController
            :decisions="decisions.filter(d => d.gate_type === 'action')"
          />
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="返回闸门 / Return Gate" name="return">
        <el-card shadow="hover">
          <GateController
            :decisions="decisions.filter(d => d.gate_type === 'return')"
          />
          <el-empty
            v-if="!decisions.filter(d => d.gate_type === 'return').length"
            description="暂无返回闸门决策记录"
          />
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
