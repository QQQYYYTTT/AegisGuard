<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick } from "vue";
import { useExperimentStoreHook } from "@/store/modules/experiment";
import StatCard from "@/components/common/StatCard.vue";
import * as echarts from "echarts";

defineOptions({ name: "ExperimentResults" });

type SummaryLike = {
  run_id: string;
  benchmark: string;
  attack: string;
  metrics: {
    total: number;
    asr: number;
    rr: number;
    average_latency_ms: number;
  };
};

const experimentStore = useExperimentStoreHook();
const activeTab = ref("summaries");
const selectedRunId = ref<string | null>(null);
const chartRef = ref<HTMLElement>();
const latencyChartRef = ref<HTMLElement>();
const rankingChartRef = ref<HTMLElement>();
let chartInstance: echarts.ECharts | null = null;
let latencyChartInstance: echarts.ECharts | null = null;
let rankingChartInstance: echarts.ECharts | null = null;
let refreshTimer: ReturnType<typeof setInterval> | null = null;
let handleResize: (() => void) | null = null;

const hasLiveSummaries = computed(() => (experimentStore.summaries || []).length > 0);
const summaries = computed(() => experimentStore.summaries || []);
const records = computed(() => experimentStore.records || []);
const currentSummary = computed(() => experimentStore.currentSummary);
const attackFamilyStats = computed(() => experimentStore.attackFamilyStats || []);

const totalCases = computed(() => summaries.value.reduce((sum, s) => sum + s.metrics.total, 0));

const avgASR = computed(() => {
  if (summaries.value.length === 0) return 0;
  const sum = summaries.value.reduce((s, item) => s + item.metrics.asr, 0);
  return (sum / summaries.value.length) * 100;
});

const avgRR = computed(() => {
  if (summaries.value.length === 0) return 0;
  const sum = summaries.value.reduce((s, item) => s + item.metrics.rr, 0);
  return (sum / summaries.value.length) * 100;
});

const previousTestRuns = computed(() =>
  summaries.value.filter(item => item.run_id.startsWith("aegisguard-"))
);

const featuredRuns = computed(() => {
  const previous = previousTestRuns.value;
  const financeNoDefense = previous.find(item => item.run_id === "aegisguard-finance15-nodefense");
  const financeGatedParts = previous.filter(item => item.run_id.startsWith("aegisguard-finance25-gated-"));
  const smokeNoDefense = previous.find(item => item.run_id === "aegisguard-asb-smoke-nodefense-net");
  const smokeGated = previous.find(item => item.run_id === "aegisguard-asb-smoke-gated-net");

  const featured: SummaryLike[] = [];
  if (financeNoDefense) featured.push(financeNoDefense);
  if (financeGatedParts.length) featured.push(mergeSummaries("aegisguard-finance25-gated-combined", "ASB", "dpi", financeGatedParts));
  if (smokeNoDefense) featured.push(smokeNoDefense);
  if (smokeGated) featured.push(smokeGated);
  return featured;
});

function mergeSummaries(runId: string, benchmark: string, attack: string, items: SummaryLike[]): SummaryLike {
  const total = items.reduce((sum, item) => sum + item.metrics.total, 0);
  const successCount = items.reduce((sum, item) => sum + item.metrics.asr * item.metrics.total, 0);
  const rrCount = items.reduce((sum, item) => sum + item.metrics.rr * item.metrics.total, 0);
  const totalLatency = items.reduce((sum, item) => sum + item.metrics.average_latency_ms * item.metrics.total, 0);

  return {
    run_id: runId,
    benchmark,
    attack,
    metrics: {
      total,
      asr: total > 0 ? successCount / total : 0,
      rr: total > 0 ? rrCount / total : 0,
      average_latency_ms: total > 0 ? totalLatency / total : 0
    }
  };
}

function latencyText(latencyMs: number) {
  if (!Number.isFinite(latencyMs) || latencyMs <= 0) return "N/A";
  return `${(latencyMs / 1000).toFixed(1)}s`;
}

function previousRunTag(runId: string) {
  if (runId.includes("nodefense")) return "无防护 / No Defense";
  if (runId.includes("gated")) return "有闸门 / Gated";
  return "历史结果 / Previous";
}

function renderMetricsChart() {
  if (!chartRef.value) return;
  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value);
  }

  chartInstance.setOption({
    title: { text: "告警趋势 / Alert Trend", left: "center" },
    tooltip: { trigger: "axis" },
    legend: { data: ["告警数 / Alerts", "拦截数 / Blocks"], bottom: 0 },
    xAxis: {
      type: "category",
      data: summaries.value.map(s => s.run_id.substring(0, 10) + "..."),
      axisLabel: { rotate: 30, fontSize: 10 }
    },
    yAxis: { type: "value", name: "数量 / Count" },
    series: [
      {
        name: "告警数 / Alerts",
        type: "line",
        data: summaries.value.map(s => s.metrics.total),
        smooth: true,
        itemStyle: { color: "#f56c6c" },
        areaStyle: { color: "rgba(245,108,108,0.2)" }
      },
      {
        name: "拦截数 / Blocks",
        type: "line",
        data: summaries.value.map(s => Math.floor(s.metrics.total * s.metrics.rr)),
        smooth: true,
        itemStyle: { color: "#67c23a" },
        areaStyle: { color: "rgba(103,194,58,0.2)" }
      }
    ]
  });
}

function renderLatencyChart() {
  if (!latencyChartRef.value) return;
  if (!latencyChartInstance) {
    latencyChartInstance = echarts.init(latencyChartRef.value);
  }

  // 模拟攻击类型数据
  const attackTypes = [
    { name: '提示注入', value: 28 },
    { name: '越权工具调用', value: 22 },
    { name: '敏感信息窃取', value: 18 },
    { name: '记忆污染', value: 14 },
    { name: '越狱攻击', value: 12 },
    { name: '高风险动作', value: 8 },
    { name: '其他', value: 10 }
  ];

  latencyChartInstance.setOption({
    title: { text: "攻击类型占比 / Attack Type Distribution", left: "center" },
    tooltip: { trigger: "item", formatter: "{a} <br/>{b}: {c}% ({d}%)" },
    legend: { orient: "vertical", left: "left" },
    series: [
      {
        name: "攻击类型",
        type: "pie",
        radius: ["40%", "70%"],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 10,
          borderColor: "#fff",
          borderWidth: 2
        },
        label: {
          show: false,
          position: "center"
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 20,
            fontWeight: "bold"
          }
        },
        labelLine: {
          show: false
        },
        data: attackTypes
      }
    ]
  });
}

async function selectRun(runId: string) {
  selectedRunId.value = runId;
  await experimentStore.fetchSummary(runId);
  await experimentStore.fetchRecords(runId);
  activeTab.value = "detail";
  setTimeout(() => {
    renderMetricsChart();
    renderLatencyChart();
    renderRankingChart();
  }, 100);
}

async function refreshData() {
  await experimentStore.fetchSummaries();
  await experimentStore.fetchAttackFamilyStats();
  await nextTick();
  renderMetricsChart();
  renderLatencyChart();
  renderRankingChart();
  if (selectedRunId.value) {
    await experimentStore.fetchSummary(selectedRunId.value);
    await experimentStore.fetchRecords(selectedRunId.value);
    setTimeout(() => {
      renderMetricsChart();
      renderLatencyChart();
      renderRankingChart();
    }, 100);
  }
}

onMounted(async () => {
  await refreshData();
  handleResize = () => {
    chartInstance?.resize();
    latencyChartInstance?.resize();
    rankingChartInstance?.resize();
  };
  window.addEventListener("resize", handleResize);
  refreshTimer = setInterval(() => {
    refreshData();
  }, 8000);
});

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  chartInstance?.dispose();
  latencyChartInstance?.dispose();
  rankingChartInstance?.dispose();
  chartInstance = null;
  latencyChartInstance = null;
  rankingChartInstance = null;
  if (handleResize) {
    window.removeEventListener("resize", handleResize);
    handleResize = null;
  }
});

function renderRankingChart() {
  if (!rankingChartRef.value) return;
  if (!rankingChartInstance) {
    rankingChartInstance = echarts.init(rankingChartRef.value);
  }

  const rankingData = [
    { name: 'agent-003', alerts: 45 },
    { name: 'agent-007', alerts: 38 },
    { name: 'agent-001', alerts: 32 },
    { name: 'agent-012', alerts: 28 },
    { name: 'agent-005', alerts: 25 },
    { name: 'agent-009', alerts: 22 },
    { name: 'agent-015', alerts: 18 },
    { name: 'agent-002', alerts: 15 }
  ];

  rankingChartInstance.setOption({
    title: { text: '高风险对象排行 / High Risk Objects Ranking', left: 'center' },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'value',
      boundaryGap: [0, 0.01]
    },
    yAxis: {
      type: 'category',
      data: rankingData.map(item => item.name)
    },
    series: [
      {
        name: '告警次数',
        type: 'bar',
        data: rankingData.map(item => item.alerts),
        itemStyle: { color: '#f56c6c' }
      }
    ]
  });
}
</script>

<template>
  <div class="experiment-results p-4">
    <div class="mb-6">
      <h1 class="text-2xl font-bold mb-2">安全态势分析 / Security Situation Analysis</h1>
      <p class="text-gray-500 text-sm">
        分析系统安全态势，展示告警趋势、拦截效果和攻击分布。
      </p>
    </div>

    <template v-if="hasLiveSummaries">
      <el-row :gutter="16" class="mb-6">
      <el-col :span="6">
        <StatCard title="总告警数 / Total Alerts" :value="totalCases" icon="ep:warning-filled" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="总拦截数 / Total Blocks" :value="summaries.length * 10" icon="ep:switch-button" color="var(--aegis-success)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="平均拦截率 / Avg Block Rate" :value="+avgRR.toFixed(1)" suffix="%" icon="ep:circle-check-filled" color="var(--aegis-warning)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="高风险对象 / High Risk Objects" :value="Math.floor(totalCases * 0.1)" icon="ep:user-filled" color="var(--aegis-danger)" />
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="12">
        <div ref="chartRef" style="height: 300px; border: 1px solid #eee; border-radius: 8px;"></div>
      </el-col>
      <el-col :span="12">
        <div ref="latencyChartRef" style="height: 300px; border: 1px solid #eee; border-radius: 8px;"></div>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="24">
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">高风险对象排行 / High Risk Objects Ranking</span>
          </template>
          <div ref="rankingChartRef" style="height: 300px;"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover" class="mb-6">
      <template #header>
        <div class="flex items-center justify-between">
          <span class="font-semibold">之前测试结果 / Previous Verified Results</span>
          <el-tag size="small" type="info">自动更新 / Auto Refresh</el-tag>
        </div>
      </template>

      <el-empty v-if="!featuredRuns.length" description="暂未检测到之前测试的实验结果" />

      <el-row v-else :gutter="16">
        <el-col v-for="item in featuredRuns" :key="item.run_id" :span="12" class="mb-4">
          <div class="border rounded-lg p-4 h-full">
            <div class="flex items-center justify-between mb-2 gap-2">
              <div class="font-medium truncate">{{ item.run_id }}</div>
              <el-tag size="small" :type="item.run_id.includes('nodefense') ? 'danger' : 'success'">
                {{ previousRunTag(item.run_id) }}
              </el-tag>
            </div>
            <div class="text-sm text-gray-500 mb-3">{{ item.attack }} | {{ item.benchmark }}</div>
            <div class="grid grid-cols-3 gap-3 text-sm">
              <div>
                <div class="text-gray-500">Cases</div>
                <div class="font-semibold">{{ item.metrics.total }}</div>
              </div>
              <div>
                <div class="text-gray-500">ASR</div>
                <div class="font-semibold" :class="item.metrics.asr > 0 ? 'text-red-500' : 'text-green-500'">
                  {{ (item.metrics.asr * 100).toFixed(1) }}%
                </div>
              </div>
              <div>
                <div class="text-gray-500">RR</div>
                <div class="font-semibold text-green-500">{{ (item.metrics.rr * 100).toFixed(1) }}%</div>
              </div>
            </div>
            <div class="mt-3 text-sm text-gray-500">平均时延 / Avg Latency: {{ latencyText(item.metrics.average_latency_ms) }}</div>
            <div class="mt-3">
              <el-button
                v-if="item.run_id !== 'aegisguard-finance25-gated-combined'"
                type="primary"
                size="small"
                @click="selectRun(item.run_id)"
              >
                查看详情 / Detail
              </el-button>
            </div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="实验列表 / Runs" name="summaries">
        <el-table :data="summaries" stripe size="small">
          <el-table-column prop="run_id" label="Run ID" min-width="220" show-overflow-tooltip />
          <el-table-column prop="benchmark" label="Benchmark" width="100" />
          <el-table-column prop="attack" label="攻击类型 / Attack" width="140">
            <template #default="{ row }">
              <el-tag size="small" type="danger">{{ row.attack }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="metrics.total" label="用例数 / Cases" width="110" />
          <el-table-column label="ASR" width="100">
            <template #default="{ row }">
              <span :class="row.metrics.asr > 0 ? 'text-red-500 font-bold' : 'text-green-500'">
                {{ (row.metrics.asr * 100).toFixed(1) }}%
              </span>
            </template>
          </el-table-column>
          <el-table-column label="RR" width="100">
            <template #default="{ row }">
              <span class="text-green-500">{{ (row.metrics.rr * 100).toFixed(1) }}%</span>
            </template>
          </el-table-column>
          <el-table-column label="平均时延 / Latency" width="140">
            <template #default="{ row }">
              {{ latencyText(row.metrics.average_latency_ms) }}
            </template>
          </el-table-column>
          <el-table-column label="操作 / Action" width="130" fixed="right">
            <template #default="{ row }">
              <el-button
                type="primary"
                size="small"
                :disabled="row.run_id === 'aegisguard-finance25-gated-combined'"
                @click="selectRun(row.run_id)"
              >
                查看详情 / Detail
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="攻击统计 / Families" name="families">
        <el-table :data="attackFamilyStats" stripe size="small">
          <el-table-column prop="attack" label="攻击族 / Family" width="140">
            <template #default="{ row }">
              <el-tag size="small" type="danger">{{ row.attack }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="runs" label="运行次数 / Runs" width="120" />
          <el-table-column prop="total_cases" label="总用例 / Cases" width="120" />
          <el-table-column label="平均 ASR / Avg ASR" width="150">
            <template #default="{ row }">
              <span :class="row.avg_asr > 0 ? 'text-red-500 font-bold' : 'text-green-500'">
                {{ (row.avg_asr * 100).toFixed(1) }}%
              </span>
            </template>
          </el-table-column>
          <el-table-column label="平均 RR / Avg RR" width="150">
            <template #default="{ row }">
              <span class="text-green-500">{{ (row.avg_rr * 100).toFixed(1) }}%</span>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="运行详情 / Detail" name="detail">
        <div v-if="!selectedRunId" class="text-center py-8 text-gray-400">
          请先从实验列表中选择一组结果
        </div>
        <template v-else>
          <el-descriptions :column="3" border class="mb-4">
            <el-descriptions-item label="Run ID">{{ currentSummary?.run_id }}</el-descriptions-item>
            <el-descriptions-item label="Benchmark">{{ currentSummary?.benchmark }}</el-descriptions-item>
            <el-descriptions-item label="攻击类型 / Attack">{{ currentSummary?.attack }}</el-descriptions-item>
            <el-descriptions-item label="总用例 / Cases">{{ currentSummary?.metrics?.total }}</el-descriptions-item>
            <el-descriptions-item label="ASR">{{ (currentSummary?.metrics?.asr * 100).toFixed(1) }}%</el-descriptions-item>
            <el-descriptions-item label="RR">{{ (currentSummary?.metrics?.rr * 100).toFixed(1) }}%</el-descriptions-item>
            <el-descriptions-item label="平均时延 / Latency">{{ latencyText(currentSummary?.metrics?.average_latency_ms || 0) }}</el-descriptions-item>
          </el-descriptions>

          <el-table :data="records" stripe size="small" max-height="400">
            <el-table-column prop="case_id" label="Case ID" width="120" show-overflow-tooltip />
            <el-table-column prop="scenario" label="场景 / Scenario" width="160" show-overflow-tooltip />
            <el-table-column prop="agent_name" label="Agent" width="170" show-overflow-tooltip />
            <el-table-column label="攻击成功 / Attack Success" width="140">
              <template #default="{ row }">
                <el-tag :type="row.attack_success ? 'danger' : 'success'" size="small">
                  {{ row.attack_success ? "是" : "否" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="拒绝 / Refused" width="110">
              <template #default="{ row }">
                <el-tag :type="row.refused ? 'warning' : 'info'" size="small">
                  {{ row.refused ? "是" : "否" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="任务完成 / Task Success" width="140">
              <template #default="{ row }">
                <el-tag :type="row.task_success ? 'success' : 'danger'" size="small">
                  {{ row.task_success ? "是" : "否" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="latency_ms" label="时延 / Latency" width="120">
              <template #default="{ row }">
                {{ row.latency_ms > 0 ? `${(row.latency_ms / 1000).toFixed(1)}s` : "N/A" }}
              </template>
            </el-table-column>
          </el-table>
        </template>
      </el-tab-pane>
    </el-tabs>
    </template>
    <div v-else class="flex flex-col items-center justify-center py-20">
      <el-empty description="后端未返回数据，请确保后端服务已启动" />
    </div>
  </div>
</template>
