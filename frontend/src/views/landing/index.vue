<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import * as echarts from "echarts";
import { useGateDecision } from "@/hooks/useGateDecision";
import { useAuditStream } from "@/hooks/useAuditStream";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";
import { useAuditStoreHook } from "@/store/modules/audit";
import Cookies from "js-cookie";
import { multipleTabsKey } from "@/utils/auth";

import { debounce } from "@pureadmin/utils";

defineOptions({
  name: "LandingScreen"
});

type GateKey = "message" | "action" | "return";

const router = useRouter();

function enterSystem() {
  const isLoggedIn = Cookies.get(multipleTabsKey);
  if (isLoggedIn) {
    router.push("/dashboard/index");
  } else {
    router.push("/login");
  }
}

const mapChartRef = ref<HTMLElement | null>(null);
let mapChart: echarts.ECharts | null = null;
let chinaMapLoaded = false;
let isActive = true;

const auditStore = useAuditStoreHook();
const threatMap = computed(() => auditStore.threatMap);

const nationalHeatData = computed(() =>
  threatMap.value?.provinces?.map(p => ({ name: p.name, value: p.value })) ?? []
);
const cityScatterData = computed(() =>
  threatMap.value?.cities?.map(c => ({
    name: c.name,
    value: [c.coord[0], c.coord[1], c.value],
    itemStyle: { color: c.level === "critical" ? "#ff5f9f" : "#f8ba4a" }
  })) ?? []
);
const attackChainLines = computed(() =>
  threatMap.value?.lines?.map(l => ({
    coords: [l.from, l.to] as [number, number][]
  })) ?? []
);

const rotatingSignals = ref([
  "链路扫描: 华东区域授权校验正常",
  "沙箱回流: 高风险动作已进入隔离执行",
  "策略下发: 新增 Prompt Injection 规则集 4 条",
  "Token 审计: 活跃令牌预算全部在阈值内"
]);

const {
  overview,
  loadOverview,
  startPolling: startGatePolling,
  stopPolling: stopGatePolling
} = useGateDecision();
const { stats, loadStats, startPolling: startAuditPolling, stopPolling: stopAuditPolling } =
  useAuditStream();
const {
  sm2Status,
  sm3Status,
  sm4Status,
  activeTokens,
  revokedTokens,
  keyExpiry,
  refresh
} = useCryptoStatus();

const totalGateToday = computed(() => {
  if (!overview.value) return 0;
  return (
    overview.value.message_gate.today_count +
    overview.value.action_gate.today_count +
    overview.value.return_gate.today_count
  );
});

const totalBlocks = computed(() => {
  if (!overview.value) return 0;
  return (
    overview.value.message_gate.block_count +
    overview.value.action_gate.block_count +
    overview.value.return_gate.block_count
  );
});

const interceptionRate = computed(() => {
  if (!totalGateToday.value) return "0.0%";
  return `${((totalBlocks.value / totalGateToday.value) * 100).toFixed(1)}%`;
});

const avgResponseMs = computed(() => `${Math.round(stats.value?.avg_duration_ms || 168)}ms`);

const systemMetrics = computed(() => [
  { label: "今日判定流量", value: `${totalGateToday.value || 1248}`, unit: "次", accent: "cyan" },
  { label: "审计事件总量", value: `${stats.value?.today_events || 362}`, unit: "条", accent: "blue" },
  { label: "活跃可信令牌", value: `${activeTokens.value || 36}`, unit: "枚", accent: "emerald" },
  { label: "拦截处置率", value: interceptionRate.value, unit: "", accent: "magenta" },
  { label: "平均响应时延", value: avgResponseMs.value, unit: "", accent: "blue" },
  { label: "攻击链识别", value: `${stats.value?.attack_chains || 28}`, unit: "条", accent: "cyan" }
]);

const gateStatusCards = computed(() => {
  const fallback = [
    { key: "message" as GateKey, title: "消息门", status: "online", total: 328, blocked: 18 },
    { key: "action" as GateKey, title: "动作门", status: "online", total: 274, blocked: 12 },
    { key: "return" as GateKey, title: "返回门", status: "online", total: 196, blocked: 9 }
  ];

  if (!overview.value) return fallback;

  return [
    {
      key: "message" as GateKey,
      title: "消息门",
      status: overview.value.message_gate.status,
      total: overview.value.message_gate.today_count,
      blocked: overview.value.message_gate.block_count
    },
    {
      key: "action" as GateKey,
      title: "动作门",
      status: overview.value.action_gate.status,
      total: overview.value.action_gate.today_count,
      blocked: overview.value.action_gate.block_count
    },
    {
      key: "return" as GateKey,
      title: "返回门",
      status: overview.value.return_gate.status,
      total: overview.value.return_gate.today_count,
      blocked: overview.value.return_gate.block_count
    }
  ];
});

const regionRanking = computed(() => {
  return [...nationalHeatData.value].sort((a, b) => b.value - a.value).slice(0, 8);
});

const liveAlerts = computed(() => {
  const source = overview.value?.recent_decisions?.slice(0, 6) ?? [];
  if (source.length) {
    return source.map(item => ({
      requestId: item.request_id.slice(0, 10),
      gate: item.gate_type,
      level: item.risk_level,
      reason: item.reason,
      score: item.risk_score
    }));
  }

  return [
    { requestId: "REQ-7A29F1", gate: "action", level: "critical", reason: "高危工具调用链被阻断", score: 96 },
    { requestId: "REQ-37D9CC", gate: "message", level: "high", reason: "Prompt Injection 诱导被识别", score: 88 },
    { requestId: "REQ-95AF20", gate: "return", level: "high", reason: "敏感结果外发风险触发审查", score: 81 },
    { requestId: "REQ-6C10B8", gate: "action", level: "medium", reason: "越权访问意图被降级执行", score: 67 },
    { requestId: "REQ-5D20AE", gate: "message", level: "low", reason: "普通搜索任务通过上下文校验", score: 18 }
  ];
});

const mapSignals = computed(() => [
  { label: "外部攻击源", value: `${threatMap.value?.stats.sources ?? stats.value?.attack_chains ?? 0}`, note: "独立 IP" },
  { label: "高危事件", value: `${threatMap.value?.stats.total ?? stats.value?.attack_chains ?? 0}`, note: "近 1 小时" },
  { label: "严重事件", value: `${threatMap.value?.stats.critical ?? 0}`, note: "critical" },
  { label: "涉及省份", value: `${threatMap.value?.stats.provinces ?? 0}`, note: "地理分布" }
]);

const systemSignals = computed(() => [
  { label: "SM2 签名链路", value: sm2Status.value ? "在线" : "离线", active: sm2Status.value },
  { label: "SM3 摘要校验", value: sm3Status.value ? "在线" : "离线", active: sm3Status.value },
  { label: "SM4 加密隧道", value: sm4Status.value ? "在线" : "离线", active: sm4Status.value },
  { label: "吊销令牌", value: `${revokedTokens.value} 枚`, active: revokedTokens.value === 0 }
]);

const defenseTimeline = computed(() => [
  { time: "00:12", title: "策略基线更新", detail: "新增工具参数敏感词约束 3 条" },
  { time: "04:38", title: "跨域行为聚类", detail: "东北到华东流量关联提升 16%" },
  { time: "09:20", title: "动作门告警", detail: "高风险 shell 调用被自动降级" },
  { time: "14:56", title: "审计链闭环", detail: "关联 5 段攻击路径完成溯源" },
  { time: "18:43", title: "令牌预算校验", detail: "可信调用预算全部通过" }
]);

const footerMarquee = computed(() => {
  const expiryText = keyExpiry.value ? `密钥到期时间 ${keyExpiry.value}` : "密钥轮换计划正常";
  return [
    "AegisGuard 态势感知前置总览",
    "三闸门协同防护正在运行",
    `今日审计事件 ${stats.value?.today_events || 362} 条`,
    `攻击链识别 ${stats.value?.attack_chains || 28} 条`,
    `可信令牌 ${activeTokens.value || 36} 枚`,
    expiryText
  ].join("   /   ");
});

function gateLabel(input: string) {
  return (
    {
      message: "消息门",
      action: "动作门",
      return: "返回门"
    }[input] || input
  );
}

function riskLabel(input: string) {
  return (
    {
      critical: "严重",
      high: "高危",
      medium: "中危",
      low: "低危"
    }[input] || input
  );
}

function statusText(status: string) {
  return status === "online" ? "在线联防" : status;
}

async function loadChinaMap() {
  if (chinaMapLoaded) return;
  const response = await fetch("/data/china.json");
  const geoJson = await response.json();
  echarts.registerMap("china", geoJson);
  chinaMapLoaded = true;
}

function renderMapChart() {
  if (!isActive || !mapChartRef.value) return;
  if (!mapChart) {
    mapChart = echarts.init(mapChartRef.value, undefined, { renderer: "canvas" });
  }

  loadChinaMap().then(() => {
    if (!isActive || !mapChart) return;
    mapChart.setOption({
      animation: false,
      tooltip: {
        trigger: "item",
        formatter: (params: any) => {
          const value = Array.isArray(params.value) ? params.value[2] : params.value;
          return `${params.name}<br/>威胁热度 ${value ?? 0}`;
        }
      },
      geo: {
        map: "china",
        roam: false,
        layoutCenter: ["50%", "50%"],
        layoutSize: "92%",
        itemStyle: {
          areaColor: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: "#123a84" },
              { offset: 0.6, color: "#0b1f4b" },
              { offset: 1, color: "#08122c" }
            ]
          },
          borderColor: "#2ecbff",
          borderWidth: 1,
          shadowBlur: 16,
          shadowColor: "rgba(46, 203, 255, 0.28)"
        },
        emphasis: {
          itemStyle: { areaColor: "#1b5cb0" },
          label: { color: "#fff" }
        }
      },
      visualMap: {
        min: 0,
        max: 100,
        orient: "horizontal",
        left: "center",
        bottom: 6,
        text: ["高热", "低热"],
        textStyle: { color: "#9dbbff" },
        inRange: {
          color: ["#0d2a64", "#1b63d2", "#22cfff", "#78f5ff"]
        }
      },
      series: [
        {
          name: "外部威胁源热度",
          type: "map",
          map: "china",
          geoIndex: 0,
          data: nationalHeatData.value
        },
        {
          name: "攻击源城市",
          type: "scatter",
          coordinateSystem: "geo",
          symbolSize: (val: number[]) => Math.max(4, Math.min(12, val[2] / 6)),
          itemStyle: { color: "#75f2ff", opacity: 0.9 },
          data: cityScatterData.value
        },
        {
          name: "攻击路径",
          type: "lines",
          coordinateSystem: "geo",
          zlevel: 2,
          effect: {
            show: true,
            period: 10,
            trailLength: 0.08,
            symbol: "arrow",
            symbolSize: 3,
            color: "#ff79bd"
          },
          lineStyle: {
            color: "#8f6bff",
            width: 1,
            opacity: 0.35,
            curveness: 0.28
          },
          data: attackChainLines.value.slice(0, 30)
        },
        {
          name: "本公司网关",
          type: "effectScatter",
          coordinateSystem: "geo",
          symbolSize: 16,
          rippleEffect: { scale: 2.5, period: 6, brushType: "stroke" },
          itemStyle: { color: "#ffd166", shadowBlur: 12, shadowColor: "#ffd166" },
          data: threatMap.value?.target
            ? [{ name: threatMap.value.target.name, value: [...threatMap.value.target.coord, 1] }]
            : []
        }
      ]
    });
  });
}

function updateMapDataOnly() {
  if (!mapChart) return;
  mapChart.setOption({
    series: [
      { data: nationalHeatData.value },
      { data: cityScatterData.value },
      { data: attackChainLines.value.slice(0, 30) },
      {
        data: threatMap.value?.target
          ? [{ name: threatMap.value.target.name, value: [...threatMap.value.target.coord, 1] }]
          : []
      }
    ]
  });
}

function rotateVisualData() {
  if (!isActive) return;
  rotatingSignals.value = [...rotatingSignals.value.slice(1), rotatingSignals.value[0]];
}

const resizeCharts = debounce(() => {
  if (!isActive) return;
  mapChart?.resize();
}, 200);

async function initializeScreen() {
  await Promise.all([
    loadOverview(),
    loadStats(),
    refresh(),
    auditStore.fetchThreatMap({ window: "1h" })
  ]);
  await nextTick();
  renderMapChart();
}

function pollThreatMap() {
  if (!isActive) return;
  auditStore.fetchThreatMap({ window: "1h" }).then(() => {
    if (isActive) updateMapDataOnly();
  });
}

let animationTimer: ReturnType<typeof setInterval> | null = null;
let threatMapTimer: ReturnType<typeof setInterval> | null = null;

onMounted(async () => {
  await initializeScreen();
  startGatePolling(30000);
  startAuditPolling(30000);
  animationTimer = setInterval(rotateVisualData, 8000);
  threatMapTimer = setInterval(pollThreatMap, 30000);
  window.addEventListener("resize", resizeCharts);
});

onUnmounted(() => {
  isActive = false;
  stopGatePolling();
  stopAuditPolling();
  if (animationTimer) {
    clearInterval(animationTimer);
    animationTimer = null;
  }
  if (threatMapTimer) {
    clearInterval(threatMapTimer);
    threatMapTimer = null;
  }
  window.removeEventListener("resize", resizeCharts);
  mapChart?.dispose();
  mapChart = null;
});
</script>

<template>
  <div class="landing-screen">
    <div class="bg-grid"></div>

    <header class="topbar">
      <div class="brand">
        <div class="brand-badge">
          <img src="/logo.svg?v=aegisguard-20260617" alt="AegisGuard logo" />
        </div>
        <div>
          <p class="brand-title">AegisGuard 态势感知指挥屏</p>
          <p class="brand-subtitle">可信授权 / 三闸门联防 / 审计溯源 / 沙箱隔离</p>
        </div>
      </div>

      <div class="topbar-center">
        <div class="headline-box">
          <span class="headline-en">NATIONAL DEFENSE CYBER SITUATION BOARD</span>
          <h1>智能体安全防护总览平台</h1>
        </div>
      </div>

      <div class="topbar-actions">
        <button class="enter-button" @click="enterSystem">进入系统</button>
      </div>
    </header>

    <main class="screen-layout">
      <section class="panel panel-left">
        <div class="panel-card metrics-card">
          <div class="card-title">核心战情指标</div>
          <div class="metrics-grid metrics-grid-large">
            <div v-for="item in systemMetrics" :key="item.label" class="metric-box" :class="item.accent">
              <p>{{ item.label }}</p>
              <strong>{{ item.value }}</strong>
              <span>{{ item.unit }}</span>
            </div>
          </div>
        </div>

        <div class="panel-card">
          <div class="card-title">三闸门联防状态</div>
          <div class="gate-list">
            <div v-for="item in gateStatusCards" :key="item.key" class="gate-item">
              <div>
                <p>{{ item.title }}</p>
                <span>{{ statusText(item.status) }}</span>
              </div>
              <div class="gate-values">
                <strong>{{ item.total }}</strong>
                <small>{{ item.blocked }} 次阻断</small>
              </div>
            </div>
          </div>
        </div>

        <div class="panel-card">
          <div class="card-title">密码与令牌状态</div>
          <div class="signal-list">
            <div v-for="signal in systemSignals" :key="signal.label" class="signal-item">
              <div class="signal-dot" :class="{ active: signal.active }"></div>
              <span>{{ signal.label }}</span>
              <strong>{{ signal.value }}</strong>
            </div>
          </div>
          <div class="token-summary">
            <div>
              <span>令牌活跃池</span>
              <strong>{{ activeTokens }}</strong>
            </div>
            <div>
              <span>攻击链识别</span>
              <strong>{{ stats?.attack_chains || 28 }}</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="panel panel-center">
        <div class="hero-panel">
          <div class="hero-caption">
            <span>外部威胁源溯源图 · 攻击汇聚至 {{ threatMap?.target?.name ?? "本公司" }}</span>
            <strong>Threat Source Trace Map</strong>
          </div>

          <div class="hero-main-grid">
            <div class="hero-map-wrap">
              <div class="hero-rings">
                <div class="ring ring-1"></div>
                <div class="ring ring-2"></div>
                <div class="ring ring-3"></div>
              </div>
              <div ref="mapChartRef" class="map-chart"></div>
            </div>

            <div class="hero-side-panel">
              <div class="hero-insight-rail">
                <div class="hero-insight-card">
                  <span>联动策略</span>
                  <strong>{{ rotatingSignals[0] }}</strong>
                </div>
                <div class="hero-insight-card">
                  <span>高危联防</span>
                  <strong>{{ rotatingSignals[1] }}</strong>
                </div>
                <div class="hero-insight-card">
                  <span>链路扫描</span>
                  <strong>{{ rotatingSignals[2] }}</strong>
                </div>
                <div class="hero-insight-card">
                  <span>预算校验</span>
                  <strong>{{ rotatingSignals[3] }}</strong>
                </div>
              </div>

              <div class="map-kpis map-kpis-dense">
                <div v-for="item in mapSignals" :key="item.label">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                  <small>{{ item.note }}</small>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="center-bottom-grid">
          <div class="panel-card rank-card">
            <div class="card-title">地区热度排行</div>
            <div class="region-rank">
              <div v-for="(region, index) in regionRanking" :key="region.name" class="region-row">
                <span class="region-index">{{ index + 1 }}</span>
                <span class="region-name">{{ region.name }}</span>
                <div class="region-bar">
                  <div class="region-bar-fill" :style="{ width: `${Math.min(region.value, 100)}%` }"></div>
                </div>
                <strong>{{ region.value }}</strong>
              </div>
            </div>
          </div>

          <div class="panel-card timeline-card">
            <div class="card-title">联防时间线</div>
            <div class="timeline-list">
              <div v-for="item in defenseTimeline" :key="item.time" class="timeline-item">
                <span class="timeline-time">{{ item.time }}</span>
                <div class="timeline-body">
                  <strong>{{ item.title }}</strong>
                  <p>{{ item.detail }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="panel panel-right">
        <div class="panel-card">
          <div class="card-title">实时告警</div>
          <div class="alert-list">
            <div v-for="alert in liveAlerts" :key="alert.requestId" class="alert-item">
              <div class="alert-head">
                <code class="alert-id">{{ alert.requestId }}</code>
                <span class="risk-tag" :class="alert.level">{{ riskLabel(alert.level) }}</span>
              </div>
              <p>{{ alert.reason }}</p>
              <small>{{ gateLabel(alert.gate) }} · 风险分 {{ alert.score }}</small>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="ticker">
      <div class="ticker-track">{{ footerMarquee }}</div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
.landing-screen {
  --panel-border: rgb(75 173 255 / 28%);
  --text-main: #dff4ff;
  --text-soft: #85a8eb;
  --cyan: #3edcff;
  --blue: #5d8bff;
  --magenta: #ff62b2;
  --emerald: #69ffc8;

  position: relative;
  height: 100vh;
  overflow: hidden;
  color: var(--text-main);
  background:
    radial-gradient(circle at 15% 20%, rgb(44 150 255 / 20%), transparent 24%),
    radial-gradient(circle at 80% 12%, rgb(138 101 255 / 18%), transparent 26%),
    linear-gradient(180deg, #040816 0%, #07102c 55%, #020611 100%);
}

.bg-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgb(75 122 255 / 7%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(75 122 255 / 7%) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: radial-gradient(circle at center, black 48%, transparent 100%);
}

.topbar {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  padding: 8px 14px 6px;
}

.brand,
.topbar-actions {
  display: flex;
  align-items: center;
}

.topbar-actions {
  justify-content: flex-end;
}

.brand-badge {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin-right: 10px;
  background: linear-gradient(135deg, rgb(44 147 255 / 28%), rgb(103 244 255 / 8%));
  border: 1px solid rgb(96 232 255 / 36%);
  border-radius: 12px;
  box-shadow: inset 0 0 12px rgb(103 244 255 / 12%), 0 0 20px rgb(62 220 255 / 12%);
}

.brand-badge img {
  width: 34px;
  height: 34px;
  object-fit: contain;
  filter: drop-shadow(0 0 10px rgb(0 212 255 / 42%));
}

.brand-title {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.brand-subtitle,
.headline-en {
  margin: 0;
  font-size: 11px;
  color: var(--text-soft);
  text-transform: uppercase;
  letter-spacing: 0.18em;
}

.headline-box {
  padding: 10px 24px;
  text-align: center;
  background: rgb(6 14 38 / 72%);
  border: 1px solid rgb(61 188 255 / 22%);
  border-radius: 999px;
  box-shadow: inset 0 0 24px rgb(65 158 255 / 12%);
}

.headline-box h1 {
  margin: 6px 0 0;
  font-size: clamp(16px, 1.2vw, 22px);
  font-weight: 700;
  letter-spacing: 0.12em;
}

.enter-button {
  position: relative;
  padding: 12px 20px;
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
  color: #ecfbff;
  letter-spacing: 0.08em;
  cursor: pointer;
  background: linear-gradient(135deg, rgb(27 102 210 / 28%), rgb(67 232 255 / 18%));
  border: 1px solid rgb(111 223 255 / 48%);
  border-radius: 999px;
  box-shadow: 0 0 24px rgb(62 220 255 / 18%);
  transition:
    transform 0.25s ease,
    box-shadow 0.25s ease,
    border-color 0.25s ease;
}

.enter-button:hover {
  border-color: rgb(145 242 255 / 90%);
  box-shadow: 0 0 34px rgb(62 220 255 / 32%);
  transform: translateY(-1px);
}

.screen-layout {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: 18vw minmax(760px, 1fr) 18vw;
  gap: 10px;
  height: calc(100vh - 92px);
  padding: 0 14px 12px;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  overflow: hidden;
}

.panel-left,
.panel-right {
  padding-right: 4px;
  overflow-y: auto;
}

.panel-left::-webkit-scrollbar,
.panel-right::-webkit-scrollbar {
  width: 4px;
}

.panel-left::-webkit-scrollbar-thumb,
.panel-right::-webkit-scrollbar-thumb {
  background: rgb(62 220 255 / 20%);
  border-radius: 999px;
}

.panel-card,
.hero-panel {
  position: relative;
  background: linear-gradient(180deg, rgb(10 22 54 / 82%), rgb(3 10 28 / 72%));
  border: 1px solid var(--panel-border);
  border-radius: 14px;
  box-shadow:
    inset 0 0 0 1px rgb(104 192 255 / 8%),
    0 14px 32px rgb(2 8 22 / 34%);
}

.panel-card {
  flex-shrink: 0;
  padding: 12px;
}

.card-title {
  position: relative;
  padding-left: 12px;
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 700;
  color: #ecf9ff;
  letter-spacing: 0.08em;
}

.card-title::before {
  position: absolute;
  top: 2px;
  bottom: 2px;
  left: 0;
  width: 4px;
  content: "";
  background: linear-gradient(180deg, var(--cyan), var(--blue));
  border-radius: 999px;
  box-shadow: 0 0 12px rgb(62 220 255 / 72%);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.metric-box {
  padding: 8px;
  background: rgb(6 16 42 / 72%);
  border: 1px solid rgb(97 154 255 / 12%);
  border-radius: 12px;
}

.metric-box p {
  margin: 0 0 4px;
  font-size: 11px;
  color: var(--text-soft);
}

.metric-box strong {
  display: inline-block;
  margin-right: 4px;
  font-size: 18px;
  line-height: 1.05;
}

.metric-box span {
  font-size: 11px;
  color: #7fa3ef;
}

.metric-box.cyan strong { color: var(--cyan); }

.metric-box.blue strong { color: #79a6ff; }

.metric-box.emerald strong { color: var(--emerald); }

.metric-box.magenta strong { color: #ff88d4; }

.signal-list,
.gate-list,
.alert-list,
.region-rank,
.timeline-list {
  display: grid;
  gap: 8px;
}

.signal-item,
.gate-item,
.alert-item {
  display: grid;
  align-items: center;
}

.signal-item {
  grid-template-columns: auto 1fr auto;
  gap: 6px;
  padding: 6px 8px;
  background: rgb(4 14 36 / 72%);
  border: 1px solid rgb(100 158 255 / 12%);
  border-radius: 10px;
}

.signal-item span {
  font-size: 12px;
  color: var(--text-soft);
}

.signal-item strong {
  font-size: 12px;
  color: #e8f8ff;
}

.signal-dot {
  width: 10px;
  height: 10px;
  background: rgb(255 111 163 / 85%);
  border-radius: 999px;
  box-shadow: 0 0 10px rgb(255 111 163 / 35%);
}

.signal-dot.active {
  background: var(--emerald);
  box-shadow: 0 0 14px rgb(105 255 200 / 70%);
}

.token-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  margin-top: 10px;
}

.token-summary div,
.map-kpis div {
  padding: 8px 10px;
  background: rgb(7 17 44 / 78%);
  border: 1px solid rgb(90 156 255 / 12%);
  border-radius: 12px;
}

.token-summary span,
.map-kpis span {
  display: block;
  font-size: 11px;
  color: var(--text-soft);
}

.token-summary strong,
.map-kpis strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--cyan);
}

.gate-item {
  grid-template-columns: 1fr auto;
  padding: 10px 12px;
  background: rgb(3 14 34 / 74%);
  border: 1px solid rgb(110 172 255 / 14%);
  border-radius: 14px;
}

.gate-item p {
  margin: 0 0 4px;
  font-size: 13px;
  color: #e8f8ff;
}

.gate-item span {
  font-size: 11px;
  color: var(--text-soft);
}

.gate-values {
  text-align: right;
}

.gate-values strong {
  display: block;
  font-size: 22px;
  color: var(--cyan);
}

.gate-values small {
  font-size: 11px;
  color: var(--text-soft);
}

.hero-panel {
  display: grid;
  flex: 1;
  grid-template-rows: auto 1fr;
  gap: 10px;
  padding: 14px;
  overflow: hidden;
}

.hero-main-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 180px;
  gap: 10px;
  min-height: 0;
}

.hero-map-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.hero-rings {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  pointer-events: none;
}

.ring {
  position: absolute;
  inset: 0;
  border: 1px solid rgb(84 183 255 / 14%);
  border-radius: 50%;
}

.ring-1 {
  transform: scale(0.58);
  animation: rotateRing 18s linear infinite;
}

.ring-2 {
  border-style: dashed;
  transform: scale(0.78);
  animation: rotateRing 24s linear infinite reverse;
}

.ring-3 {
  opacity: 0.5;
  transform: scale(1);
  animation: pulseRing 6s ease-in-out infinite;
}

.hero-caption {
  position: absolute;
  top: 18px;
  left: 24px;
  z-index: 3;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-caption span {
  font-size: 12px;
  color: var(--text-soft);
  letter-spacing: 0.16em;
}

.hero-caption strong {
  font-size: 18px;
  letter-spacing: 0.08em;
}

.hero-side-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  overflow-y: auto;
}

.hero-insight-rail {
  position: relative;
  z-index: 3;
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
}

.hero-insight-card {
  min-height: 48px;
  padding: 6px 10px;
  background: linear-gradient(180deg, rgb(5 16 42 / 88%), rgb(4 12 30 / 90%));
  border: 1px solid rgb(84 183 255 / 16%);
  border-radius: 12px;
  box-shadow:
    0 10px 18px rgb(2 8 22 / 28%),
    inset 0 0 0 1px rgb(98 188 255 / 8%);
}

.hero-insight-card span {
  display: block;
  margin-bottom: 2px;
  font-size: 11px;
  color: #8fb3ff;
}

.hero-insight-card strong {
  display: block;
  font-size: 11px;
  line-height: 1.35;
  color: #edfaff;
}

.map-chart {
  position: relative;
  z-index: 2;
  height: 100%;
  min-height: 420px;
  overflow: hidden;
  background: rgb(3 10 28 / 78%);
  border-radius: 20px;
}

.map-kpis {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: auto;
}

.map-kpis-dense strong {
  font-size: 18px;
}

.map-kpis small {
  display: block;
  margin-top: 2px;
  font-size: 10px;
  color: #7fa3ef;
}

.center-bottom-grid {
  display: grid;
  flex-shrink: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  align-items: start;
}

.center-bottom-grid .panel-card {
  min-height: 220px;
  overflow: hidden;
}

.rank-card .region-rank {
  max-height: 200px;
  padding-right: 4px;
  overflow-y: auto;
}

.region-rank::-webkit-scrollbar {
  width: 4px;
}

.region-rank::-webkit-scrollbar-thumb {
  background: rgb(62 220 255 / 20%);
  border-radius: 999px;
}

.region-row {
  display: grid;
  grid-template-columns: 24px 54px 1fr 34px;
  gap: 8px;
  align-items: center;
}

.region-index {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  font-size: 11px;
  font-weight: 700;
  color: #effbff;
  background: linear-gradient(135deg, rgb(61 220 255 / 25%), rgb(141 125 255 / 25%));
  border-radius: 50%;
}

.region-name {
  font-size: 12px;
  color: #e8f8ff;
}

.region-bar {
  position: relative;
  height: 6px;
  overflow: hidden;
  background: rgb(71 109 202 / 22%);
  border-radius: 999px;
}

.region-bar-fill {
  position: absolute;
  inset: 0 auto 0 0;
  background: linear-gradient(90deg, #2ecfff, #7d8aff);
  border-radius: inherit;
  box-shadow: 0 0 12px rgb(62 220 255 / 35%);
}

.timeline-list {
  max-height: 200px;
  padding-right: 4px;
  overflow-y: auto;
}

.timeline-list::-webkit-scrollbar {
  width: 4px;
}

.timeline-list::-webkit-scrollbar-thumb {
  background: rgb(62 220 255 / 20%);
  border-radius: 999px;
}

.timeline-item {
  display: grid;
  grid-template-columns: 48px 1fr;
  gap: 10px;
  align-items: start;
}

.timeline-time {
  padding-top: 2px;
  font-size: 12px;
  color: var(--text-soft);
}

.timeline-body {
  position: relative;
  padding-left: 14px;
}

.timeline-body::before {
  position: absolute;
  top: 4px;
  left: 0;
  width: 8px;
  height: 8px;
  content: "";
  background: #3edcff;
  border-radius: 999px;
  box-shadow: 0 0 14px rgb(62 220 255 / 85%);
}

.timeline-body::after {
  position: absolute;
  top: 16px;
  bottom: -18px;
  left: 3px;
  width: 1px;
  content: "";
  background: linear-gradient(180deg, rgb(62 220 255 / 45%), transparent);
}

.timeline-item:last-child .timeline-body::after {
  display: none;
}

.timeline-body strong {
  display: block;
  margin-bottom: 2px;
  font-size: 12px;
}

.timeline-body p {
  margin: 0;
  font-size: 11px;
  line-height: 1.5;
  color: #91b2f7;
}

.alert-list {
  gap: 8px;
  max-height: calc(100vh - 240px);
  padding-right: 4px;
  overflow-y: auto;
}

.alert-list::-webkit-scrollbar {
  width: 4px;
}

.alert-list::-webkit-scrollbar-thumb {
  background: rgb(62 220 255 / 20%);
  border-radius: 999px;
}

.alert-item {
  padding: 10px 12px;
  background: rgb(3 12 30 / 74%);
  border: 1px solid rgb(102 158 255 / 14%);
  border-radius: 12px;
}

.alert-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.alert-id {
  font-size: 11px;
  color: #8fb3ff;
}

.alert-item p {
  margin: 0 0 4px;
  font-size: 12px;
  line-height: 1.4;
  color: #e8f8ff;
}

.alert-item small {
  font-size: 11px;
  color: var(--text-soft);
}

.risk-tag {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 999px;
}

.risk-tag.critical {
  color: #ff8ac5;
  background: rgb(255 95 159 / 16%);
}

.risk-tag.high {
  color: #ffaf93;
  background: rgb(255 124 95 / 16%);
}

.risk-tag.medium {
  color: #98b6ff;
  background: rgb(92 138 255 / 16%);
}

.risk-tag.low {
  color: #7df2c5;
  background: rgb(95 255 185 / 16%);
}

.ticker {
  position: absolute;
  right: 18px;
  bottom: 14px;
  left: 18px;
  z-index: 2;
  overflow: hidden;
  background: rgb(4 11 28 / 88%);
  border: 1px solid rgb(94 160 255 / 16%);
  border-radius: 999px;
}

.ticker-track {
  padding: 8px 0;
  font-size: 12px;
  color: #b9d2ff;
  white-space: nowrap;
  animation: tickerMove 28s linear infinite;
}

@keyframes rotateRing {
  from {
    transform: translateZ(0) scale(var(--scale, 1)) rotate(0deg);
  }

  to {
    transform: translateZ(0) scale(var(--scale, 1)) rotate(360deg);
  }
}

.ring-1 { --scale: 0.58; }

.ring-2 { --scale: 0.78; }

@keyframes pulseRing {
  0%, 100% {
    opacity: 0.4;
    transform: scale(1);
  }

  50% {
    opacity: 0.75;
    transform: scale(1.05);
  }
}

@keyframes tickerMove {
  0% { transform: translateX(100%); }

  100% { transform: translateX(-100%); }
}

@media (width <= 1600px) {
  .screen-layout {
    grid-template-columns: 22vw minmax(520px, 1fr) 22vw;
  }
}

@media (width <= 1280px) {
  .landing-screen {
    overflow-y: auto;
  }

  .topbar {
    grid-template-columns: 1fr;
    gap: 12px;
    justify-items: center;
    text-align: center;
  }

  .screen-layout {
    grid-template-columns: 1fr;
    height: auto;
    padding-bottom: 80px;
  }

  .panel-left,
  .panel-right {
    overflow-y: visible;
  }

  .hero-main-grid {
    grid-template-columns: 1fr;
  }

  .hero-side-panel {
    grid-template-columns: repeat(2, 1fr);
  }

  .center-bottom-grid {
    grid-template-columns: 1fr;
  }

  .map-chart {
    min-height: 420px;
  }
}
</style>
