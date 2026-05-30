<script setup lang="ts">
import { ref, onMounted } from "vue";
import StatCard from "@/components/common/StatCard.vue";
import * as echarts from "echarts";

defineOptions({ name: "AuthCenterIndex" });

// 告警数据
const highRiskAlerts = ref(12);
const mediumRiskAlerts = ref(25);
const lowRiskAlerts = ref(45);

const alertTypeData = ref([
  { name: '提示注入', value: 18 },
  { name: '越权工具调用', value: 14 },
  { name: '敏感信息窃取', value: 11 },
  { name: '记忆污染', value: 8 },
  { name: '越狱攻击', value: 9 },
  { name: '高风险动作', value: 7 },
  { name: '其他', value: 15 },
]);

const alertTrendData = ref([
  { time: '01-10', high: 5, medium: 8, low: 12 },
  { time: '01-11', high: 3, medium: 10, low: 15 },
  { time: '01-12', high: 8, medium: 12, low: 18 },
  { time: '01-13', high: 6, medium: 9, low: 14 },
  { time: '01-14', high: 10, medium: 15, low: 20 },
  { time: '01-15', high: 12, medium: 25, low: 45 },
]);

const alertsList = ref([
  {
    id: 1,
    time: '2024-01-15 14:30:25',
    level: '高',
    type: '提示注入',
    source: 'agent-003',
    gate: '消息闸门',
    authTriggered: true,
    sandboxTriggered: false,
    description: '检测到提示注入：试图绕过系统指令并泄露System Prompt'
  },
  {
    id: 2,
    time: '2024-01-15 14:25:10',
    level: '中',
    type: '越权工具调用',
    source: 'agent-005',
    gate: '动作闸门',
    authTriggered: false,
    sandboxTriggered: true,
    description: 'Agent试图调用 code_exec 工具，超出授权范围'
  },
  {
    id: 3,
    time: '2024-01-15 14:20:45',
    level: '高',
    type: '敏感信息窃取',
    source: 'agent-001',
    gate: '返回闸门',
    authTriggered: true,
    sandboxTriggered: true,
    description: '返回结果中包含API密钥和数据库凭证，已被沙箱隔离'
  },
  {
    id: 4,
    time: '2024-01-15 14:15:30',
    level: '低',
    type: '记忆污染',
    source: 'agent-002',
    gate: '消息闸门',
    authTriggered: false,
    sandboxTriggered: false,
    description: '消息试图向Agent长期记忆写入恶意指令'
  },
  {
    id: 5,
    time: '2024-01-15 14:10:15',
    level: '中',
    type: '越狱攻击',
    source: 'agent-004',
    gate: '动作闸门',
    authTriggered: false,
    sandboxTriggered: true,
    description: '检测到DAN越狱模式，Agent试图绕过安全限制执行高风险操作'
  },
]);

const alertTypeChartRef = ref<HTMLElement>();
const alertTrendChartRef = ref<HTMLElement>();
let alertTypeChart: echarts.ECharts | null = null;
let alertTrendChart: echarts.ECharts | null = null;

function renderAlertTypeChart() {
  if (!alertTypeChartRef.value) return;
  if (!alertTypeChart) {
    alertTypeChart = echarts.init(alertTypeChartRef.value);
  }

  alertTypeChart.setOption({
    title: { text: '告警类型分布 / Alert Type Distribution', left: 'center' },
    tooltip: { trigger: 'item' },
    legend: { orient: 'vertical', left: 'left' },
    series: [
      {
        name: '告警类型',
        type: 'pie',
        radius: '50%',
        data: alertTypeData.value.map(item => ({
          value: item.value,
          name: item.name
        })),
        emphasis: {
          itemStyle: {
            shadowBlur: 10,
            shadowOffsetX: 0,
            shadowColor: 'rgba(0, 0, 0, 0.5)'
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

  alertTrendChart.setOption({
    title: { text: '告警时间趋势 / Alert Time Trend', left: 'center' },
    tooltip: { trigger: 'axis' },
    legend: { data: ['高风险', '中风险', '低风险'], bottom: 0 },
    xAxis: {
      type: 'category',
      data: alertTrendData.value.map(d => d.time)
    },
    yAxis: { type: 'value' },
    series: [
      {
        name: '高风险',
        type: 'bar',
        stack: 'total',
        data: alertTrendData.value.map(d => d.high),
        itemStyle: { color: '#f56c6c' }
      },
      {
        name: '中风险',
        type: 'bar',
        stack: 'total',
        data: alertTrendData.value.map(d => d.medium),
        itemStyle: { color: '#e6a23c' }
      },
      {
        name: '低风险',
        type: 'bar',
        stack: 'total',
        data: alertTrendData.value.map(d => d.low),
        itemStyle: { color: '#67c23a' }
      }
    ]
  });
}

onMounted(() => {
  renderAlertTypeChart();
  renderAlertTrendChart();
});

function viewAlertDetail(alert: any) {
  // TODO: 实现告警详情查看
  console.log('View alert detail:', alert);
}
</script>

<template>
  <div class="auth-center p-4">
    <h1 class="text-2xl font-bold mb-4">风险告警中心 / Risk Alert Center</h1>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="8">
        <StatCard
          title="高风险告警 / High Risk Alerts"
          :value="highRiskAlerts"
          icon="ep:warning-filled"
          color="var(--aegis-danger)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="中风险告警 / Medium Risk Alerts"
          :value="mediumRiskAlerts"
          icon="ep:warning"
          color="var(--aegis-warning)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="低风险告警 / Low Risk Alerts"
          :value="lowRiskAlerts"
          icon="ep:info-filled"
          color="var(--aegis-info)"
        />
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-6">
      <el-col :span="12">
        <el-card shadow="hover">
          <div ref="alertTypeChartRef" style="height: 300px;"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <div ref="alertTrendChartRef" style="height: 300px;"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover">
      <template #header>
        <span class="font-semibold">告警列表 / Alert List</span>
      </template>
      <el-table :data="alertsList" stripe size="small">
        <el-table-column prop="time" label="时间 / Time" width="160" />
        <el-table-column prop="level" label="级别 / Level" width="100">
          <template #default="{ row }">
            <el-tag :type="row.level === '高' ? 'danger' : row.level === '中' ? 'warning' : 'info'" size="small">
              {{ row.level }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型 / Type" width="120" />
        <el-table-column prop="source" label="来源 / Source" width="140" />
        <el-table-column prop="gate" label="命中闸门 / Gate Hit" width="120">
          <template #default="{ row }">
            <el-tag size="small" type="info">{{ row.gate }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可信授权 / Auth" width="100">
          <template #default="{ row }">
            <el-tag :type="row.authTriggered ? 'success' : 'info'" size="small">
              {{ row.authTriggered ? '触发' : '未触发' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="记忆沙箱 / Sandbox" width="120">
          <template #default="{ row }">
            <el-tag :type="row.sandboxTriggered ? 'warning' : 'info'" size="small">
              {{ row.sandboxTriggered ? '触发' : '未触发' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述 / Description" show-overflow-tooltip />
        <el-table-column label="操作 / Action" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="viewAlertDetail(row)">
              详情 / Detail
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
