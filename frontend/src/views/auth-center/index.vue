<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from "vue";
import { useRouter } from "vue-router";
import StatCard from "@/components/common/StatCard.vue";
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import { useGateStoreHook } from "@/store/modules/gate";
import { useAuditStoreHook } from "@/store/modules/audit";
import * as echarts from "echarts";
import type { GateDecision } from "@/api/gate";

defineOptions({ name: "AuthCenterIndex" });

const gateStore = useGateStoreHook();
const auditStore = useAuditStoreHook();
const router = useRouter();

const loading = ref(false);

const demoAlertDecisions: GateDecision[] = [
  {
    request_id: "demo-alert-dpi-001",
    timestamp: new Date(Date.now() - 12 * 60_000).toISOString(),
    gate_type: "message",
    decision: "Block",
    risk_score: 100,
    risk_level: "critical",
    matched_rules: ["DPI_DIRECT_PROMPT_INJECTION", "SYSTEM_OVERRIDE"],
    reason: "DPI 直接提示注入，试图覆盖系统策略。",
    agent_id: "openclaw"
  },
  {
    request_id: "demo-alert-opi-002",
    timestamp: new Date(Date.now() - 28 * 60_000).toISOString(),
    gate_type: "return",
    decision: "Degrade",
    risk_score: 65,
    risk_level: "high",
    matched_rules: ["OPI_EXTERNAL_CONTENT", "RETURN_GATE_SANITIZE"],
    reason: "OPI 外部观察结果夹带隐藏指令。",
    agent_id: "openclaw"
  },
  {
    request_id: "demo-alert-pot-003",
    timestamp: new Date(Date.now() - 45 * 60_000).toISOString(),
    gate_type: "action",
    decision: "HumanApproval",
    risk_score: 97,
    risk_level: "critical",
    matched_rules: ["POT_PRIVILEGED_TOOL", "MISSING_TOKEN"],
    reason: "POT 权限工具诱导，缺少可信授权令牌。",
    tool_name: "AdminTransferTool",
    agent_id: "openclaw"
  }
];

const alertDecisions = computed(() =>
  gateStore.decisions?.length ? gateStore.decisions : demoAlertDecisions
);

const highRiskCount = computed(() => {
  const list = alertDecisions.value;
  return list.filter(d => d.risk_level === "high" || d.risk_level === "critical").length;
});

const mediumRiskCount = computed(() => {
  const list = alertDecisions.value;
  return list.filter(d => d.risk_level === "medium").length;
});

const lowRiskCount = computed(() => {
  const list = alertDecisions.value;
  return list.filter(d => d.risk_level === "low").length;
});

const alertTypeData = computed(() => {
  const ruleCounts: Record<string, number> = {};
  alertDecisions.value.forEach(d => {
    (d.matched_rules || []).forEach(rule => {
      ruleCounts[rule] = (ruleCounts[rule] || 0) + 1;
    });
  });
  if (Object.keys(ruleCounts).length === 0) return [];
  return Object.entries(ruleCounts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([name, value]) => ({ name, value }));
});

const alertTrendData = computed(() => {
  const now = new Date();
  const buckets: Record<string, { high: number; medium: number; low: number }> = {};
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now.getTime() - i * 24 * 3600000);
    const key = `${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    buckets[key] = { high: 0, medium: 0, low: 0 };
  }
  const keys = Object.keys(buckets);
  alertDecisions.value.forEach(d => {
    const ts = new Date(d.timestamp);
    const eventKey = `${String(ts.getMonth() + 1).padStart(2, "0")}-${String(ts.getDate()).padStart(2, "0")}`;
    if (buckets[eventKey]) {
      if (d.risk_level === "high" || d.risk_level === "critical") buckets[eventKey].high++;
      else if (d.risk_level === "medium") buckets[eventKey].medium++;
      else buckets[eventKey].low++;
    }
  });
  return keys.map(time => ({
    time,
    high: buckets[time].high,
    medium: buckets[time].medium,
    low: buckets[time].low
  }));
});

const alertListData = computed(() => {
  return alertDecisions.value.slice(0, 50).map(d => ({
    id: d.request_id,
    time: d.timestamp,
    level: d.risk_level === "critical" ? "critical" : d.risk_level,
    levelLabel: d.risk_level === "critical" ? "严重" : d.risk_level === "high" ? "高" : d.risk_level === "medium" ? "中" : "低",
    type: d.gate_type === "message" ? "消息检测" : d.gate_type === "action" ? "动作检测" : "返回检测",
    source: d.agent_id || d.request_id?.slice(0, 12) || "unknown",
    gate: d.gate_type,
    riskScore: d.risk_score,
    decision: d.decision,
    reason: d.reason || `${d.gate_type} 门: ${d.decision} (评分 ${d.risk_score})`,
    requestId: d.request_id
  }));
});

const alertTypeChartRef = ref<HTMLElement>();
const alertTrendChartRef = ref<HTMLElement>();
let alertTypeChart: echarts.ECharts | null = null;
let alertTrendChart: echarts.ECharts | null = null;
let handleResize: (() => void) | null = null;

function renderAlertTypeChart() {
  if (!alertTypeChartRef.value) return;
  if (!alertTypeChart) {
    alertTypeChart = echarts.init(alertTypeChartRef.value);
  }
  const data = alertTypeData.value.length > 0
    ? alertTypeData.value
    : [{ name: "暂无数据", value: 1 }];

  alertTypeChart.setOption({
    title: {
      text: "告警类型分布 / Alert Type Distribution",
      left: "center",
      textStyle: { color: "#e8f8ff", fontWeight: 700 }
    },
    tooltip: {
      trigger: "item",
      backgroundColor: "rgba(4, 18, 38, 0.94)",
      borderColor: "#1d9bf0",
      textStyle: { color: "#d9f4ff" }
    },
    legend: {
      orient: "vertical",
      left: "left",
      textStyle: { color: "#b7dfff", fontWeight: 600 }
    },
    series: [
      {
        name: "告警类型",
        type: "pie",
        radius: "50%",
        data: data.map(item => ({
          value: item.value,
          name: item.name
        })),
        label: { color: "#d9f4ff", fontWeight: 600 },
        labelLine: { lineStyle: { color: "#8fb6d8" } },
        emphasis: {
          itemStyle: {
            shadowBlur: 10,
            shadowOffsetX: 0,
            shadowColor: "rgba(0, 0, 0, 0.5)"
          }
        }
      }
    ]
  });
}

function renderAlertTrendChart() {
  if (!alertTrendChartRef.value) return;
  if (!alertTrendChart) {
    alertTrendChart = echarts.init(alertTrendChartRef.value);
  }
  const data = alertTrendData.value.length > 0
    ? alertTrendData.value
    : [{ time: "暂无", high: 0, medium: 0, low: 0 }];

  alertTrendChart.setOption({
    title: {
      text: "告警时间趋势 / Alert Time Trend",
      left: "center",
      textStyle: { color: "#e8f8ff", fontWeight: 700 }
    },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(4, 18, 38, 0.94)",
      borderColor: "#1d9bf0",
      textStyle: { color: "#d9f4ff" }
    },
    legend: {
      data: ["高风险", "中风险", "低风险"],
      bottom: 0,
      textStyle: { color: "#b7dfff", fontWeight: 600 }
    },
    xAxis: {
      type: "category",
      data: data.map(d => d.time),
      axisLine: { lineStyle: { color: "rgba(159, 223, 255, 0.52)" } },
      axisLabel: { color: "#b7dfff", fontWeight: 600 }
    },
    yAxis: {
      type: "value",
      axisLine: { lineStyle: { color: "rgba(159, 223, 255, 0.52)" } },
      axisLabel: { color: "#b7dfff", fontWeight: 600 },
      splitLine: { lineStyle: { color: "rgba(159, 223, 255, 0.18)" } }
    },
    series: [
      {
        name: "高风险",
        type: "bar",
        stack: "total",
        data: data.map(d => d.high),
        itemStyle: { color: "#f56c6c" }
      },
      {
        name: "中风险",
        type: "bar",
        stack: "total",
        data: data.map(d => d.medium),
        itemStyle: { color: "#e6a23c" }
      },
      {
        name: "低风险",
        type: "bar",
        stack: "total",
        data: data.map(d => d.low),
        itemStyle: { color: "#67c23a" }
      }
    ]
  });
}

async function loadData() {
  loading.value = true;
  try {
    await Promise.all([
      gateStore.fetchOverview(),
      gateStore.fetchDecisions(),
      auditStore.fetchLogs(),
      auditStore.fetchStats()
    ]);
  } finally {
    loading.value = false;
  }
  await nextTick();
  renderAlertTypeChart();
  renderAlertTrendChart();
}

function goToAlertDetail(row: { requestId?: string }) {
  if (row.requestId) {
    router.push({ path: "/audit-trace/index", query: { chain: row.requestId } });
  } else {
    router.push({ path: "/gate-control/index" });
  }
}

onMounted(async () => {
  await loadData();
  handleResize = () => {
    alertTypeChart?.resize();
    alertTrendChart?.resize();
  };
  window.addEventListener("resize", handleResize);
});

onUnmounted(() => {
  alertTypeChart?.dispose();
  alertTrendChart?.dispose();
  alertTypeChart = null;
  alertTrendChart = null;
  if (handleResize) {
    window.removeEventListener("resize", handleResize);
    handleResize = null;
  }
});
</script>

<template>
  <div class="auth-center p-4">
    <h1 class="text-2xl font-bold mb-4">风险告警中心 / Risk Alert Center</h1>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="8">
        <StatCard
          title="高风险告警"
          :value="highRiskCount"
          icon="ep:warning-filled"
          color="var(--aegis-danger)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="中风险告警"
          :value="mediumRiskCount"
          icon="ep:warning"
          color="var(--aegis-warning)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="低风险告警"
          :value="lowRiskCount"
          icon="ep:info-filled"
          color="var(--aegis-info)"
        />
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="12">
        <el-card shadow="hover">
          <div v-loading="loading" ref="alertTypeChartRef" style="height: 300px;"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <div v-loading="loading" ref="alertTrendChartRef" style="height: 300px;"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover">
      <template #header>
        <div class="flex items-center justify-between">
          <span class="font-semibold">告警列表 / Alert List</span>
          <el-button size="small" :loading="loading" @click="loadData">
            刷新
          </el-button>
        </div>
      </template>
      <el-table :data="alertListData" stripe size="small" @row-click="goToAlertDetail" style="cursor: pointer;">
        <el-table-column prop="time" label="时间 / Time" width="160">
          <template #default="{ row }">
            {{ new Date(row.time).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column prop="levelLabel" label="级别 / Level" width="80">
          <template #default="{ row }">
            <el-tag
              :type="row.level === 'critical' || row.level === 'high' ? 'danger' : row.level === 'medium' ? 'warning' : 'info'"
              size="small"
            >
              {{ row.levelLabel }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="决策 / Decision" width="100">
          <template #default="{ row }">
            <DecisionBadge :decision="row.decision" />
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型 / Type" width="120" />
        <el-table-column prop="source" label="来源 / Source" width="140" />
        <el-table-column label="风险分数" width="90">
          <template #default="{ row }">
            <RiskBadge :level="row.level" />
            <span class="text-xs ml-1">{{ row.riskScore }}%</span>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="描述 / Description" show-overflow-tooltip />
      </el-table>
      <div v-if="alertListData.length === 0" class="py-8 text-center text-gray-400">
        暂无告警数据
      </div>
    </el-card>
  </div>
</template>
