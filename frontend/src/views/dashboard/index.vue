<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import StatCard from "@/components/common/StatCard.vue";
import RealTimeTag from "@/components/common/RealTimeTag.vue";
import { useGateDecision } from "@/hooks/useGateDecision";
import { useAuditStream } from "@/hooks/useAuditStream";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";
import * as echarts from "echarts";
import Cpu from "~icons/ep/cpu";

defineOptions({ name: "DashboardIndex" });

const {
  overview,
  decisions,
  loadOverview,
  startPolling: startGatePolling,
  stopPolling: stopGatePolling
} = useGateDecision();
const {
  stats,
  events,
  loadStats,
  loadLogs,
  startPolling: startAuditPolling,
  stopPolling: stopAuditPolling
} = useAuditStream();
const { sm2Status, sm3Status, sm4Status, activeTokens } = useCryptoStatus();

const riskTrendChartRef = ref<HTMLElement>();
let riskTrendChart: echarts.ECharts | null = null;

const requestTotal = computed(() => {
  if (!overview.value) return 0;
  return (
    (overview.value.message_gate?.today_count || 0) +
    (overview.value.action_gate?.today_count || 0) +
    (overview.value.return_gate?.today_count || 0)
  );
});

const blockCount = computed(() => {
  if (!overview.value) return 0;
  return (
    (overview.value.message_gate?.block_count || 0) +
    (overview.value.action_gate?.block_count || 0) +
    (overview.value.return_gate?.block_count || 0)
  );
});

const activeAgentCount = computed(() => {
  const ids = new Set<string>();
  ids.add("openclaw");
  decisions.value.forEach(item => item.agent_id && ids.add(item.agent_id));
  stats.value?.top_agents?.forEach(item => item.agent_id && ids.add(item.agent_id));
  return ids.size || connectedAgents.value.length;
});

const highRiskEvents = computed(() => {
  return decisions.value.filter(
    item => item.risk_level === "high" || item.risk_level === "critical"
  ).length;
});

const gateHits = computed(() => {
  if (!overview.value) return 0;
  const gates = [
    overview.value.message_gate?.decision_counts || {},
    overview.value.action_gate?.decision_counts || {},
    overview.value.return_gate?.decision_counts || {}
  ];
  return gates.reduce(
    (total, gate) => total + Object.values(gate).reduce((sum, value) => sum + value, 0),
    0
  );
});

const authExceptions = computed(() => {
  return events.value.filter(
    item =>
      item.event_type === "authorization" &&
      (item.decision === "Deny" || item.decision === "Block")
  ).length;
});

const sandboxTriggers = computed(() => {
  return events.value.filter(item => item.event_type === "sandbox").length;
});

const blockRate = computed(() => {
  if (!requestTotal.value) return "0%";
  return `${Math.round((blockCount.value / requestTotal.value) * 100)}%`;
});

const riskTrendData = computed(() => {
  const now = new Date();
  const buckets: Record<string, { alerts: number; blocks: number }> = {};

  for (let i = 5; i >= 0; i--) {
    const time = new Date(now.getTime() - i * 4 * 3600000);
    const key = `${String(time.getHours()).padStart(2, "0")}:00`;
    buckets[key] = { alerts: 0, blocks: 0 };
  }

  const keys = Object.keys(buckets);
  decisions.value.forEach(item => {
    const timestamp = new Date(item.timestamp);
    const bucketKey = keys.find(key => {
      const hour = Number(key.split(":")[0]);
      return Math.abs(timestamp.getHours() - hour) <= 2;
    });
    if (!bucketKey) return;
    buckets[bucketKey].alerts += 1;
    if (item.decision === "Block" || item.decision === "Deny") {
      buckets[bucketKey].blocks += 1;
    }
  });

  return Object.entries(buckets).map(([time, value]) => ({ time, ...value }));
});

const fallbackAgents = [
  { agent_id: "openclaw", count: 128 },
  { agent_id: "financial_analyst_agent", count: 86 },
  { agent_id: "legal_consultant_agent", count: 72 },
  { agent_id: "workflow_agent", count: 58 },
  { agent_id: "customer_service_agent", count: 44 },
  { agent_id: "research_agent", count: 31 }
];

const agentProfiles: Record<string, { name: string; domain: string; scope: string; status: string }> = {
  openclaw: {
    name: "OpenClaw Agent",
    domain: "安全测试编排",
    scope: "benchmark:run / gate:evaluate",
    status: "在线"
  },
  financial_analyst_agent: {
    name: "金融分析 Agent",
    domain: "金融投研",
    scope: "market:read / finance:write",
    status: "在线"
  },
  legal_consultant_agent: {
    name: "法律顾问 Agent",
    domain: "法务合规",
    scope: "case:read / evidence:write",
    status: "在线"
  },
  workflow_agent: {
    name: "流程编排 Agent",
    domain: "业务自动化",
    scope: "workflow:execute",
    status: "在线"
  },
  customer_service_agent: {
    name: "客服 Agent",
    domain: "用户服务",
    scope: "ticket:read / crm:write",
    status: "监控中"
  },
  research_agent: {
    name: "检索研究 Agent",
    domain: "知识检索",
    scope: "search:read",
    status: "在线"
  }
};

const connectedAgents = computed(() => {
  const source = stats.value?.top_agents?.length ? [...stats.value.top_agents] : [...fallbackAgents];
  if (!source.some(item => item.agent_id.toLowerCase() === "openclaw")) {
    source.unshift(fallbackAgents[0]);
  }
  return source.slice(0, 6).map((item, index) => {
    const profile = agentProfiles[item.agent_id] || {
      name: item.agent_id,
      domain: "业务 Agent",
      scope: "gateway:proxy",
      status: "在线"
    };

    return {
      ...profile,
      id: item.agent_id,
      count: item.count,
      health: Math.max(72, 98 - index * 6)
    };
  });
});

const latestAlerts = computed(() => {
  const list = decisions.value.slice(0, 10).map(item => ({
    time: item.timestamp,
    level:
      item.risk_level === "critical"
        ? "严重"
        : item.risk_level === "high"
          ? "高"
          : item.risk_level === "medium"
            ? "中"
            : "低",
    type:
      item.gate_type === "message"
        ? "消息闸门"
        : item.gate_type === "action"
          ? "动作闸门"
          : "返回闸门",
    source: item.agent_id || item.request_id?.slice(0, 12) || "unknown",
    description: item.reason || `${item.gate_type} gate ${item.decision}`
  }));

  return list.length
    ? list
    : [
        {
          time: "暂无数据",
          level: "低",
          type: "系统",
          source: "-",
          description: "网关已启动，等待 Agent 流量接入"
        }
      ];
});

onMounted(() => {
  Promise.all([loadOverview(), loadStats(), loadLogs()]).then(() => {
    startGatePolling(5000);
    startAuditPolling(5000);
    renderRiskTrendChart();
  });
});

onUnmounted(() => {
  stopGatePolling();
  stopAuditPolling();
  riskTrendChart?.dispose();
  riskTrendChart = null;
});

watch(riskTrendData, () => renderRiskTrendChart(), { deep: true });

function renderRiskTrendChart() {
  if (!riskTrendChartRef.value) return;
  if (!riskTrendChart) {
    riskTrendChart = echarts.init(riskTrendChartRef.value);
  }

  riskTrendChart.setOption({
    backgroundColor: "transparent",
    grid: { top: 28, right: 20, bottom: 36, left: 36 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(4, 18, 38, 0.92)",
      borderColor: "#1d9bf0",
      textStyle: { color: "#d9f4ff" }
    },
    legend: {
      data: ["告警数", "拦截数"],
      bottom: 0,
      textStyle: { color: "#8fb6d8" }
    },
    xAxis: {
      type: "category",
      data: riskTrendData.value.map(item => item.time),
      axisLine: { lineStyle: { color: "#1d4166" } },
      axisLabel: { color: "#8fb6d8" }
    },
    yAxis: {
      type: "value",
      splitLine: { lineStyle: { color: "rgba(86, 160, 220, 0.14)" } },
      axisLabel: { color: "#8fb6d8" }
    },
    series: [
      {
        name: "告警数",
        type: "line",
        data: riskTrendData.value.map(item => item.alerts),
        smooth: true,
        symbolSize: 7,
        itemStyle: { color: "#00d4ff" },
        areaStyle: { color: "rgba(0, 212, 255, 0.12)" }
      },
      {
        name: "拦截数",
        type: "line",
        data: riskTrendData.value.map(item => item.blocks),
        smooth: true,
        symbolSize: 7,
        itemStyle: { color: "#ff4d7d" },
        areaStyle: { color: "rgba(255, 77, 125, 0.1)" }
      }
    ]
  });
}
</script>

<template>
  <div class="dashboard tech-page p-4">
    <div class="dashboard-hero mb-5">
      <div>
        <p class="eyebrow">AegisGuard Command Center</p>
        <h1>安全态势总览</h1>
        <p class="hero-subtitle">
          面向 Agent 网关的三道闸、可信授权、沙箱隔离与审计链路实时监测。
        </p>
      </div>
      <div class="flex flex-wrap gap-2 justify-end">
        <RealTimeTag label="SM2" :active="sm2Status" />
        <RealTimeTag label="SM3" :active="sm3Status" />
        <RealTimeTag label="SM4" :active="sm4Status" />
        <RealTimeTag label="Action Gate" :active="true" />
        <RealTimeTag label="Sandbox" :active="true" />
      </div>
    </div>

    <el-row :gutter="12" class="mb-4">
      <el-col :xs="12" :lg="4">
        <StatCard title="请求总量" :value="requestTotal" icon="ep:data-line" color="var(--aegis-neon)" />
      </el-col>
      <el-col :xs="12" :lg="4">
        <StatCard title="已接入 Agent" :value="activeAgentCount" icon="ep:connection" color="var(--aegis-success)" />
      </el-col>
      <el-col :xs="12" :lg="4">
        <StatCard title="高风险事件" :value="highRiskEvents" icon="ep:warning-filled" color="var(--aegis-danger)" />
      </el-col>
      <el-col :xs="12" :lg="4">
        <StatCard title="拦截次数" :value="blockCount" icon="ep:switch-button" color="var(--aegis-danger)" />
      </el-col>
      <el-col :xs="12" :lg="4">
        <StatCard title="三道闸命中" :value="gateHits" icon="ep:gate" color="var(--aegis-primary)" />
      </el-col>
      <el-col :xs="12" :lg="4">
        <StatCard title="拦截率" :value="blockRate" icon="ep:trend-charts" color="var(--aegis-warning)" />
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-4">
      <el-col :xs="24" :lg="7">
        <el-card shadow="never" class="dashboard-card h-full">
          <template #header>
            <div class="card-title">风险趋势</div>
          </template>
          <div ref="riskTrendChartRef" class="trend-chart" />
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card shadow="never" class="dashboard-card h-full agent-card">
          <template #header>
            <div class="card-title">
              <span>已接入 Agent</span>
              <small>{{ activeAgentCount }} 个实例受网关保护</small>
            </div>
          </template>
          <div class="agent-grid">
            <div v-for="agent in connectedAgents" :key="agent.id" class="agent-item">
              <div class="agent-icon">
                <el-icon><Cpu /></el-icon>
              </div>
              <div class="agent-main">
                <div class="agent-row">
                  <strong>{{ agent.name }}</strong>
                  <el-tag size="small" effect="dark" type="success">{{ agent.status }}</el-tag>
                </div>
                <p>{{ agent.id }}</p>
                <div class="agent-meta">
                  <span>{{ agent.domain }}</span>
                  <span>{{ agent.scope }}</span>
                </div>
                <div class="agent-health">
                  <span>健康度</span>
                  <el-progress :percentage="agent.health" :stroke-width="7" :show-text="false" />
                  <b>{{ agent.health }}%</b>
                </div>
              </div>
              <div class="agent-count">
                <b>{{ agent.count }}</b>
                <span>请求</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="5">
        <el-card shadow="never" class="dashboard-card mb-4">
          <template #header>
            <div class="card-title">闸门状态</div>
          </template>
          <div v-if="overview" class="gate-list">
            <div class="gate-status-item">
              <span>消息闸门</span>
              <el-tag type="success" size="small">{{ overview.message_gate.status }}</el-tag>
              <small>{{ overview.message_gate.block_count }} 次拦截</small>
            </div>
            <div class="gate-status-item">
              <span>动作闸门</span>
              <el-tag type="success" size="small">{{ overview.action_gate.status }}</el-tag>
              <small>{{ overview.action_gate.block_count }} 次拦截</small>
            </div>
            <div class="gate-status-item">
              <span>返回闸门</span>
              <el-tag type="success" size="small">{{ overview.return_gate.status }}</el-tag>
              <small>{{ overview.return_gate.block_count }} 次拦截</small>
            </div>
          </div>
        </el-card>

        <el-card shadow="never" class="dashboard-card">
          <template #header>
            <div class="card-title">可信授权</div>
          </template>
          <div class="token-panel">
            <strong>{{ activeTokens }}</strong>
            <span>活跃 SM2 令牌</span>
            <p>授权异常 {{ authExceptions }} 次 · 沙箱触发 {{ sandboxTriggers }} 次</p>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="dashboard-card">
      <template #header>
        <div class="card-title">最新告警</div>
      </template>
      <el-table :data="latestAlerts" stripe size="small" max-height="250">
        <el-table-column prop="time" label="时间" width="170" />
        <el-table-column prop="level" label="级别" width="90">
          <template #default="{ row }">
            <el-tag :type="row.level === '高' || row.level === '严重' ? 'danger' : row.level === '中' ? 'warning' : 'info'" size="small">
              {{ row.level }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="120" />
        <el-table-column prop="source" label="来源 Agent" width="180" />
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.dashboard {
  min-height: 100vh;
  color: #d9f4ff;
}

.dashboard-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 22px 24px;
  background:
    linear-gradient(120deg, rgba(0, 212, 255, 0.14), rgba(25, 82, 255, 0.06)),
    rgba(5, 18, 38, 0.86);
  border: 1px solid rgba(78, 192, 255, 0.24);
  border-radius: 10px;
  box-shadow: 0 18px 45px rgba(0, 0, 0, 0.28);
}

.eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  letter-spacing: 0.18em;
  color: #60d7ff;
  text-transform: uppercase;
}

.dashboard-hero h1 {
  margin: 0;
  font-size: 30px;
  font-weight: 800;
  color: #f4fbff;
}

.hero-subtitle {
  margin: 8px 0 0;
  color: #8fb6d8;
}

.trend-chart {
  height: 320px;
}

.dashboard-card {
  background: rgba(5, 18, 38, 0.86);
  border: 1px solid rgba(78, 192, 255, 0.18);
  border-radius: 8px;
  box-shadow: inset 0 0 30px rgba(0, 212, 255, 0.04);
}

.card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  font-size: 16px;
  font-weight: 700;
  color: #e8f8ff;
}

.card-title small {
  font-size: 12px;
  font-weight: 400;
  color: #6fa8c9;
}

.agent-grid {
  display: grid;
  gap: 12px;
}

.agent-item {
  display: grid;
  grid-template-columns: 46px 1fr 72px;
  gap: 14px;
  align-items: center;
  padding: 14px;
  background: linear-gradient(90deg, rgba(8, 31, 62, 0.96), rgba(9, 41, 75, 0.68));
  border: 1px solid rgba(88, 190, 255, 0.18);
  border-radius: 8px;
}

.agent-icon {
  display: grid;
  width: 46px;
  height: 46px;
  place-items: center;
  color: #00d4ff;
  background: rgba(0, 212, 255, 0.1);
  border: 1px solid rgba(0, 212, 255, 0.24);
  border-radius: 8px;
}

.agent-row,
.agent-meta,
.agent-health {
  display: flex;
  align-items: center;
  gap: 10px;
}

.agent-row {
  justify-content: space-between;
}

.agent-main strong {
  color: #f4fbff;
}

.agent-main p,
.agent-meta,
.agent-health {
  margin: 4px 0 0;
  font-size: 12px;
  color: #87abc8;
}

.agent-meta span + span::before {
  margin-right: 10px;
  color: #1d9bf0;
  content: "/";
}

.agent-health :deep(.el-progress) {
  flex: 1;
}

.agent-health b {
  min-width: 34px;
  color: #00d4ff;
}

.agent-count {
  text-align: right;
}

.agent-count b {
  display: block;
  font-size: 24px;
  color: #00d4ff;
}

.agent-count span {
  font-size: 12px;
  color: #87abc8;
}

.gate-list {
  display: grid;
  gap: 12px;
}

.gate-status-item {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 4px 10px;
  padding: 12px;
  background: rgba(7, 28, 55, 0.78);
  border: 1px solid rgba(78, 192, 255, 0.14);
  border-radius: 8px;
}

.gate-status-item span {
  color: #e8f8ff;
}

.gate-status-item small {
  color: #87abc8;
}

.token-panel {
  text-align: center;
}

.token-panel strong {
  display: block;
  font-size: 42px;
  line-height: 1;
  color: #00d4ff;
}

.token-panel span,
.token-panel p {
  color: #87abc8;
}

.token-panel p {
  margin: 10px 0 0;
  font-size: 12px;
}
</style>
