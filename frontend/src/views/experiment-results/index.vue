<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useExperimentStoreHook } from "@/store/modules/experiment";
import StatCard from "@/components/common/StatCard.vue";
import * as echarts from "echarts";

defineOptions({ name: "ExperimentResults" });

const experimentStore = useExperimentStoreHook();
const activeTab = ref("summaries");
const selectedRunId = ref<string | null>(null);
const chartRef = ref<HTMLElement>();
const latencyChartRef = ref<HTMLElement>();
let chartInstance: echarts.ECharts | null = null;
let latencyChartInstance: echarts.ECharts | null = null;
let refreshTimer: ReturnType<typeof setInterval> | null = null;

const summaries = computed(() => experimentStore.summaries || []);
const records = computed(() => experimentStore.records || []);
const currentSummary = computed(() => experimentStore.currentSummary);
const attackFamilyStats = computed(() => experimentStore.attackFamilyStats || []);

const totalCases = computed(() =>
  summaries.value.reduce((sum, s) => sum + s.metrics.total, 0)
);

const avgASR = computed(() => {
  if (summaries.value.length === 0) return 0;
  const sum = summaries.value.reduce((s, item) => s + item.metrics.asr, 0);
  return (sum / summaries.value.length * 100).toFixed(1);
});

const avgRR = computed(() => {
  if (summaries.value.length === 0) return 0;
  const sum = summaries.value.reduce((s, item) => s + item.metrics.rr, 0);
  return (sum / summaries.value.length * 100).toFixed(1);
});

function renderMetricsChart() {
  if (!chartRef.value) return;
  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value);
  }

  chartInstance.setOption({
    title: { text: "ASR / RR 对比 | ASR and RR Comparison", left: "center" },
    tooltip: { trigger: "axis" },
    legend: { data: ["ASR (%)", "RR (%)"], bottom: 0 },
    xAxis: {
      type: "category",
      data: summaries.value.map(s => s.run_id),
      axisLabel: { rotate: 30, fontSize: 10 }
    },
    yAxis: { type: "value", max: 100, name: "百分比 (%)" },
    series: [
      {
        name: "ASR (%)",
        type: "bar",
        data: summaries.value.map(s => +(s.metrics.asr * 100).toFixed(1)),
        itemStyle: { color: "#f56c6c" }
      },
      {
        name: "RR (%)",
        type: "bar",
        data: summaries.value.map(s => +(s.metrics.rr * 100).toFixed(1)),
        itemStyle: { color: "#67c23a" }
      }
    ]
  });
}

function renderLatencyChart() {
  if (!latencyChartRef.value) return;
  if (!latencyChartInstance) {
    latencyChartInstance = echarts.init(latencyChartRef.value);
  }

  latencyChartInstance.setOption({
    title: { text: "平均时延 | Average Latency", left: "center" },
    tooltip: { trigger: "axis", formatter: "{b}: {c}s" },
    xAxis: {
      type: "category",
      data: summaries.value.map(s => s.run_id),
      axisLabel: { rotate: 30, fontSize: 10 }
    },
    yAxis: { type: "value", name: "时延 (秒)" },
    series: [
      {
        name: "Latency",
        type: "line",
        data: summaries.value.map(s => +(s.metrics.average_latency_ms / 1000).toFixed(1)),
        smooth: true,
        itemStyle: { color: "#409eff" },
        areaStyle: { color: "rgba(64,158,255,0.2)" }
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
  }, 100);
}

async function refreshData() {
  await experimentStore.fetchSummaries();
  await experimentStore.fetchAttackFamilyStats();
  if (selectedRunId.value) {
    await experimentStore.fetchSummary(selectedRunId.value);
    await experimentStore.fetchRecords(selectedRunId.value);
    setTimeout(() => {
      renderMetricsChart();
      renderLatencyChart();
    }, 100);
  }
}

onMounted(async () => {
  await refreshData();
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
  chartInstance = null;
  latencyChartInstance = null;
});
</script>

<template>
  <div class="experiment-results p-4">
    <div class="mb-6">
      <h1 class="text-2xl font-bold mb-2">实验结果可视化 / Experiment Results</h1>
      <p class="text-gray-500 text-sm">ASB 基准测试结果汇总与分析</p>
    </div>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="6">
        <StatCard title="实验运行数 / Runs" :value="summaries.length" icon="ep:data-line" color="var(--aegis-primary)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="总测试用例 / Cases" :value="totalCases" icon="ep:document" color="var(--aegis-info)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="平均 ASR / Avg ASR" :value="Number(avgASR)" suffix="%" icon="ep:warning-filled" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="平均 RR / Avg RR" :value="Number(avgRR)" suffix="%" icon="ep:circle-check-filled" color="var(--aegis-warning)" />
      </el-col>
    </el-row>

    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="实验列表 / Runs" name="summaries">
        <el-table :data="summaries" stripe size="small">
          <el-table-column prop="run_id" label="Run ID" min-width="200" show-overflow-tooltip />
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
              {{ (row.metrics.average_latency_ms / 1000).toFixed(1) }}s
            </template>
          </el-table-column>
          <el-table-column label="操作 / Action" width="130" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" @click="selectRun(row.run_id)">查看详情 / Detail</el-button>
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
            <el-descriptions-item label="平均时延 / Latency">{{ (currentSummary?.metrics?.average_latency_ms / 1000).toFixed(1) }}s</el-descriptions-item>
          </el-descriptions>

          <el-row :gutter="16" class="mb-4">
            <el-col :span="12">
              <div ref="chartRef" style="height: 300px; border: 1px solid #eee; border-radius: 8px;"></div>
            </el-col>
            <el-col :span="12">
              <div ref="latencyChartRef" style="height: 300px; border: 1px solid #eee; border-radius: 8px;"></div>
            </el-col>
          </el-row>

          <el-table :data="records" stripe size="small" max-height="400">
            <el-table-column prop="case_id" label="Case ID" width="120" show-overflow-tooltip />
            <el-table-column prop="scenario" label="场景 / Scenario" width="140" show-overflow-tooltip />
            <el-table-column prop="agent_name" label="Agent" width="150" show-overflow-tooltip />
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
                {{ (row.latency_ms / 1000).toFixed(1) }}s
              </template>
            </el-table-column>
          </el-table>
        </template>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
