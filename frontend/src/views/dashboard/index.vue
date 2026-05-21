<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import StatCard from "@/components/common/StatCard.vue";
import RealTimeTag from "@/components/common/RealTimeTag.vue";
import { useGateDecision } from "@/hooks/useGateDecision";
import { useAuditStream } from "@/hooks/useAuditStream";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";
import * as echarts from "echarts";

defineOptions({ name: "DashboardIndex" });

const { overview, decisions, loadOverview, startPolling: startGatePolling, stopPolling: stopGatePolling } = useGateDecision();
const { stats, loadStats, startPolling: startAuditPolling, stopPolling: stopAuditPolling } = useAuditStream();
const { sm2Status, sm3Status, sm4Status, activeTokens } = useCryptoStatus();

const riskTrendChartRef = ref<HTMLElement>();
const riskMapChartRef = ref<HTMLElement>();
let riskTrendChart: echarts.ECharts | null = null;
let riskMapChart: echarts.ECharts | null = null;
let chinaMapLoaded = false;

const requestTotal = ref(12543);
const concurrency = ref(234);
const activeIPs = ref(89);
const activeHosts = ref(45);
const highRiskEvents = ref(12);
const blockCount = ref(113);
const gateHits = ref(67);
const authExceptions = ref(3);
const sandboxTriggers = ref(8);
const mapLoadError = ref(false);

onMounted(async () => {
  await Promise.all([loadOverview(), loadStats()]);
  startGatePolling(5000);
  startAuditPolling(5000);
  renderRiskTrendChart();
  renderRiskMapChart();
});

onUnmounted(() => {
  stopGatePolling();
  stopAuditPolling();
  riskTrendChart?.dispose();
  riskMapChart?.dispose();
  riskTrendChart = null;
  riskMapChart = null;
});

// 风险趋势数据
const riskTrendData = ref([
  { time: '00:00', alerts: 5, blocks: 3 },
  { time: '04:00', alerts: 8, blocks: 5 },
  { time: '08:00', alerts: 12, blocks: 8 },
  { time: '12:00', alerts: 15, blocks: 10 },
  { time: '16:00', alerts: 18, blocks: 12 },
  { time: '20:00', alerts: 10, blocks: 7 },
]);

// 攻击来源数据
const riskRegionData = ref([
  { name: '北京', value: 82 },
  { name: '上海', value: 65 },
  { name: '广东', value: 95 },
  { name: '浙江', value: 68 },
  { name: '江苏', value: 74 },
  { name: '四川', value: 53 },
  { name: '湖北', value: 48 },
  { name: '福建', value: 39 },
  { name: '安徽', value: 32 },
  { name: '天津', value: 28 }
]);

// 最新告警数据
const latestAlerts = ref([
  { time: '2024-01-15 14:30:25', level: '高', type: 'SQL注入', source: '192.168.1.100', description: '检测到可疑SQL注入攻击' },
  { time: '2024-01-15 14:25:10', level: '中', type: 'XSS攻击', source: '192.168.1.105', description: '发现跨站脚本攻击尝试' },
  { time: '2024-01-15 14:20:45', level: '高', type: 'DDoS攻击', source: '10.0.0.50', description: '检测到大规模DDoS攻击' },
  { time: '2024-01-15 14:15:30', level: '低', type: '扫描探测', source: '192.168.1.120', description: '端口扫描活动' },
  { time: '2024-01-15 14:10:15', level: '中', type: '暴力破解', source: '192.168.1.110', description: 'SSH暴力破解尝试' },
]);

function renderRiskTrendChart() {
  if (!riskTrendChartRef.value) return;
  if (!riskTrendChart) {
    riskTrendChart = echarts.init(riskTrendChartRef.value);
  }

  riskTrendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['告警数 / Alerts', '拦截数 / Blocks'], bottom: 0 },
    xAxis: {
      type: 'category',
      data: riskTrendData.value.map(d => d.time)
    },
    yAxis: { type: 'value' },
    series: [
      {
        name: '告警数 / Alerts',
        type: 'line',
        data: riskTrendData.value.map(d => d.alerts),
        smooth: true,
        itemStyle: { color: '#f56c6c' }
      },
      {
        name: '拦截数 / Blocks',
        type: 'line',
        data: riskTrendData.value.map(d => d.blocks),
        smooth: true,
        itemStyle: { color: '#67c23a' }
      }
    ]
  });
}

async function loadChinaMapJson() {
  if (chinaMapLoaded) return;
  try {
    const response = await fetch('/data/china.json');
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const geoJson = await response.json();
    echarts.registerMap('china', geoJson);
    chinaMapLoaded = true;
  } catch (error) {
    console.warn('China map load failed:', error);
    mapLoadError.value = true;
  }
}

async function renderRiskMapChart() {
  if (!riskMapChartRef.value) return;
  if (!riskMapChart) {
    riskMapChart = echarts.init(riskMapChartRef.value);
  }

  await loadChinaMapJson();

  const option = {
    tooltip: {
      trigger: 'item',
      formatter: '{b}<br/>风险评分: {c}'
    },
    visualMap: {
      min: 0,
      max: 100,
      left: 'left',
      bottom: '15%',
      text: ['高', '低'],
      calculable: true,
      inRange: {
        color: ['#e0f3f8', '#74add1', '#4575b4', '#313695']
      }
    },
    series: [
      {
        name: '风险评分',
        type: 'map',
        map: 'china',
        roam: true,
        emphasis: {
          label: {
            show: true
          }
        },
        data: riskRegionData.value
      }
    ]
  };

  riskMapChart.setOption(option);
}
</script>

<template>
  <div class="dashboard p-4 bg-white text-slate-900" style="min-height: 100vh;">
    <!-- Header -->
    <div class="mb-8">
      <div class="flex items-center justify-between mb-4">
        <h1 class="text-3xl font-bold text-slate-900">智能体安全防护总览平台 / Intelligent Security Protection Platform</h1>
        <div class="flex gap-3">
          <RealTimeTag label="SM2" :active="sm2Status" />
          <RealTimeTag label="SM3" :active="sm3Status" />
          <RealTimeTag label="SM4" :active="sm4Status" />
          <RealTimeTag label="Action Gate" :active="true" />
          <RealTimeTag label="Sandbox" :active="true" />
        </div>
      </div>
    </div>

    <!-- Top KPI Cards - 6 columns compact -->
    <el-row :gutter="12" class="mb-6">
      <el-col :span="4">
        <StatCard
          title="请求总量"
          :value="requestTotal"
          icon="ep:data-line"
          color="var(--aegis-primary)"
        />
      </el-col>
      <el-col :span="4">
        <StatCard
          title="并发数"
          :value="concurrency"
          icon="ep:trend-charts"
          color="var(--aegis-info)"
        />
      </el-col>
      <el-col :span="4">
        <StatCard
          title="活跃 IP"
          :value="activeIPs"
          icon="ep:ip"
          color="var(--aegis-success)"
        />
      </el-col>
      <el-col :span="4">
        <StatCard
          title="活跃主机"
          :value="activeHosts"
          icon="ep:monitor"
          color="var(--aegis-warning)"
        />
      </el-col>
      <el-col :span="4">
        <StatCard
          title="高风险事件"
          :value="highRiskEvents"
          icon="ep:warning-filled"
          color="var(--aegis-danger)"
        />
      </el-col>
      <el-col :span="4">
        <StatCard
          title="拦截次数"
          :value="blockCount"
          icon="ep:shield"
          color="var(--aegis-danger)"
        />
      </el-col>
    </el-row>

    <!-- System Feature Cards - 3 columns -->
    <el-row :gutter="12" class="mb-8">
      <el-col :span="8">
        <StatCard
          title="三级闸门命中次数"
          :value="gateHits"
          icon="ep:gate"
          color="var(--aegis-primary)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="可信授权异常数"
          :value="authExceptions"
          icon="ep:lock"
          color="var(--aegis-warning)"
        />
      </el-col>
      <el-col :span="8">
        <StatCard
          title="记忆沙箱触发数"
          :value="sandboxTriggers"
          icon="ep:box"
          color="var(--aegis-info)"
        />
      </el-col>
    </el-row>

    <!-- Main Content - Map centered, side panels -->
    <el-row :gutter="16" class="mb-6">
      <!-- Left Panel -->
      <el-col :span="6">
        <el-card shadow="hover" class="dashboard-card h-full">
          <template #header>
            <div class="card-title">风险趋势 / Risk Trend</div>
          </template>
          <div ref="riskTrendChartRef" style="height: 280px;"></div>
        </el-card>
      </el-col>

      <!-- Central Map - Large and Prominent -->
      <el-col :span="13">
        <el-card shadow="hover" class="dashboard-card h-full">
          <template #header>
            <div class="card-title text-center flex-1">威胁地域分布 / Threat Geographic Distribution</div>
          </template>
          <div v-if="mapLoadError" class="flex items-center justify-center h-96 text-slate-500">
            地图加载失败，请检查网络或使用本地地图资源。
          </div>
          <div v-else ref="riskMapChartRef" style="height: 450px; width: 100%;"></div>
        </el-card>
      </el-col>

      <!-- Right Panel -->
      <el-col :span="5">
        <el-card shadow="hover" class="dashboard-card mb-4">
          <template #header>
            <div class="card-title">闸门状态 / Gate Status</div>
          </template>
          <div v-if="overview" class="space-y-3">
            <div class="gate-status-item">
              <div class="flex items-center justify-between">
                <span class="text-sm text-slate-700">消息闸门</span>
                <el-tag type="success" size="small">{{ overview.message_gate.status }}</el-tag>
              </div>
              <div class="text-xs text-slate-500 mt-1">
                {{ overview.message_gate.block_count }} 次拦截
              </div>
            </div>
            <div class="gate-status-item">
              <div class="flex items-center justify-between">
                <span class="text-sm text-slate-700">动作闸门</span>
                <el-tag type="success" size="small">{{ overview.action_gate.status }}</el-tag>
              </div>
              <div class="text-xs text-slate-500 mt-1">
                {{ overview.action_gate.block_count }} 次拦截
              </div>
            </div>
            <div class="gate-status-item">
              <div class="flex items-center justify-between">
                <span class="text-sm text-slate-700">返回闸门</span>
                <el-tag type="success" size="small">{{ overview.return_gate.status }}</el-tag>
              </div>
              <div class="text-xs text-slate-500 mt-1">
                {{ overview.return_gate.block_count }} 次拦截
              </div>
            </div>
          </div>
        </el-card>
        <el-card shadow="hover" class="dashboard-card">
          <template #header>
            <div class="card-title">活跃令牌 / Active Tokens</div>
          </template>
          <div class="text-center">
            <div class="text-4xl font-bold text-cyan-400">{{ activeTokens }}</div>
            <div class="text-xs text-slate-500 mt-2">当前活跃的 SM2 令牌</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Bottom Alert Section -->
    <el-row :gutter="16">
      <el-col :span="24">
        <el-card shadow="hover" class="dashboard-card">
          <template #header>
            <div class="card-title">最新告警 / Latest Alerts</div>
          </template>
          <el-table :data="latestAlerts" stripe size="small" max-height="250" :default-sort="{ prop: 'time', order: 'descending' }">
            <el-table-column prop="time" label="时间 / Time" width="160" />
            <el-table-column prop="level" label="级别 / Level" width="80">
              <template #default="{ row }">
                <el-tag :type="row.level === '高' ? 'danger' : row.level === '中' ? 'warning' : 'info'" size="small">
                  {{ row.level }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="type" label="类型 / Type" width="120" />
            <el-table-column prop="source" label="来源 / Source" width="140" />
            <el-table-column prop="description" label="描述 / Description" show-overflow-tooltip />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.dashboard {
  background-color: #ffffff;
}
.dashboard .dashboard-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 18px;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.06);
}
.dashboard .dashboard-card .el-card__body {
  padding: 20px;
}
.dashboard .card-title {
  font-size: 16px;
  font-weight: 700;
  color: #111827;
}
.dashboard .text-cyan-400 {
  color: #0ea5e9 !important;
}
.dashboard .el-tag {
  font-size: 12px;
}
</style>
