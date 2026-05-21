<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import * as echarts from "echarts";
import { useGateDecision } from "@/hooks/useGateDecision";
import { useAuditStream } from "@/hooks/useAuditStream";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";

defineOptions({
  name: "LandingScreen"
});

type Particle = {
  top: string;
  left: string;
  size: string;
  delay: string;
  duration: string;
};

type RegionHeat = {
  name: string;
  value: number;
};

type GateKey = "message" | "action" | "return";

const router = useRouter();

const mapChartRef = ref<HTMLElement | null>(null);
const trendChartRef = ref<HTMLElement | null>(null);
const decisionChartRef = ref<HTMLElement | null>(null);
const funnelChartRef = ref<HTMLElement | null>(null);
const radarChartRef = ref<HTMLElement | null>(null);
const healthChartRef = ref<HTMLElement | null>(null);

let mapChart: echarts.ECharts | null = null;
let trendChart: echarts.ECharts | null = null;
let decisionChart: echarts.ECharts | null = null;
let funnelChart: echarts.ECharts | null = null;
let radarChart: echarts.ECharts | null = null;
let healthChart: echarts.ECharts | null = null;
let chinaMapLoaded = false;
let animationTimer: ReturnType<typeof setInterval> | null = null;

const particles: Particle[] = [
  { top: "10%", left: "8%", size: "5px", delay: "0s", duration: "10s" },
  { top: "18%", left: "30%", size: "8px", delay: "1s", duration: "13s" },
  { top: "14%", left: "75%", size: "6px", delay: "3s", duration: "12s" },
  { top: "32%", left: "10%", size: "10px", delay: "2s", duration: "15s" },
  { top: "26%", left: "88%", size: "4px", delay: "4s", duration: "11s" },
  { top: "40%", left: "22%", size: "7px", delay: "1.5s", duration: "9s" },
  { top: "46%", left: "70%", size: "12px", delay: "0.5s", duration: "16s" },
  { top: "58%", left: "6%", size: "6px", delay: "5s", duration: "14s" },
  { top: "52%", left: "92%", size: "8px", delay: "3.5s", duration: "10s" },
  { top: "67%", left: "28%", size: "9px", delay: "1.2s", duration: "12s" },
  { top: "74%", left: "58%", size: "5px", delay: "2.4s", duration: "9s" },
  { top: "82%", left: "16%", size: "11px", delay: "0.2s", duration: "14s" },
  { top: "86%", left: "80%", size: "7px", delay: "4.1s", duration: "10s" },
  { top: "24%", left: "50%", size: "6px", delay: "5.3s", duration: "17s" },
  { top: "12%", left: "58%", size: "9px", delay: "2.2s", duration: "15s" },
  { top: "64%", left: "84%", size: "5px", delay: "3.2s", duration: "11s" }
];

const nationalHeatData = ref<RegionHeat[]>([
  { name: "北京", value: 91 },
  { name: "上海", value: 88 },
  { name: "广东", value: 97 },
  { name: "浙江", value: 82 },
  { name: "江苏", value: 79 },
  { name: "山东", value: 74 },
  { name: "四川", value: 68 },
  { name: "湖北", value: 65 },
  { name: "陕西", value: 59 },
  { name: "福建", value: 70 },
  { name: "河南", value: 62 },
  { name: "天津", value: 57 },
  { name: "重庆", value: 54 },
  { name: "辽宁", value: 50 }
]);

const cityScatterData = ref([
  { name: "北京", value: [116.4, 39.9, 91] },
  { name: "上海", value: [121.47, 31.23, 88] },
  { name: "深圳", value: [114.05, 22.55, 97] },
  { name: "杭州", value: [120.15, 30.28, 82] },
  { name: "南京", value: [118.78, 32.04, 79] },
  { name: "武汉", value: [114.3, 30.59, 65] },
  { name: "成都", value: [104.06, 30.67, 68] },
  { name: "西安", value: [108.94, 34.34, 59] }
]);

const attackLinks = ref([
  [{ coord: [87.62, 43.82] }, { coord: [116.4, 39.9] }],
  [{ coord: [126.63, 45.75] }, { coord: [121.47, 31.23] }],
  [{ coord: [91.11, 29.97] }, { coord: [114.05, 22.55] }],
  [{ coord: [111.67, 40.82] }, { coord: [120.15, 30.28] }],
  [{ coord: [103.84, 36.06] }, { coord: [118.78, 32.04] }],
  [{ coord: [112.55, 37.87] }, { coord: [113.26, 23.13] }]
]);

const trendHours = ref(["00:00", "04:00", "08:00", "12:00", "16:00", "20:00", "24:00"]);
const trendAlerts = ref([18, 28, 46, 63, 75, 58, 34]);
const trendBlocks = ref([8, 16, 30, 43, 57, 46, 22]);

const stageData = ref([
  { name: "诱导探测", value: 182 },
  { name: "Prompt 注入", value: 127 },
  { name: "越权调用", value: 91 },
  { name: "敏感外发", value: 54 },
  { name: "自动阻断", value: 39 }
]);

const radarValues = ref([92, 86, 89, 84, 95, 78]);

const healthServices = ["Auth", "Token", "Message", "Action", "Return", "Audit"];
const healthMetrics = ["签名", "延迟", "拦截", "链路", "恢复", "可信"];
const healthMatrix = ref([
  [0, 0, 96],
  [0, 1, 85],
  [0, 2, 91],
  [0, 3, 88],
  [0, 4, 84],
  [0, 5, 93],
  [1, 0, 94],
  [1, 1, 82],
  [1, 2, 90],
  [1, 3, 86],
  [1, 4, 80],
  [1, 5, 92],
  [2, 0, 97],
  [2, 1, 89],
  [2, 2, 95],
  [2, 3, 93],
  [2, 4, 88],
  [2, 5, 94],
  [3, 0, 95],
  [3, 1, 84],
  [3, 2, 92],
  [3, 3, 90],
  [3, 4, 83],
  [3, 5, 89],
  [4, 0, 93],
  [4, 1, 80],
  [4, 2, 88],
  [4, 3, 87],
  [4, 4, 81],
  [4, 5, 86],
  [5, 0, 96],
  [5, 1, 87],
  [5, 2, 90],
  [5, 3, 92],
  [5, 4, 89],
  [5, 5, 97]
]);

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
const {
  stats,
  loadStats,
  startPolling: startAuditPolling,
  stopPolling: stopAuditPolling
} = useAuditStream();
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
  {
    label: "今日判定流量",
    value: `${totalGateToday.value || 1248}`,
    unit: "次",
    accent: "cyan"
  },
  {
    label: "审计事件总量",
    value: `${stats.value?.today_events || 362}`,
    unit: "条",
    accent: "blue"
  },
  {
    label: "活跃可信令牌",
    value: `${activeTokens.value || 36}`,
    unit: "枚",
    accent: "emerald"
  },
  {
    label: "拦截处置率",
    value: interceptionRate.value,
    unit: "",
    accent: "magenta"
  },
  {
    label: "平均响应时延",
    value: avgResponseMs.value,
    unit: "",
    accent: "blue"
  },
  {
    label: "攻击链识别",
    value: `${stats.value?.attack_chains || 28}`,
    unit: "条",
    accent: "cyan"
  }
]);

const gateStatusCards = computed(() => {
  const fallback: Array<{
    key: GateKey;
    title: string;
    status: string;
    total: number;
    blocked: number;
  }> = [
    { key: "message", title: "消息门", status: "online", total: 328, blocked: 18 },
    { key: "action", title: "动作门", status: "online", total: 274, blocked: 12 },
    { key: "return", title: "返回门", status: "online", total: 196, blocked: 9 }
  ];

  if (!overview.value) return fallback;

  return [
    {
      key: "message" as const,
      title: "消息门",
      status: overview.value.message_gate.status,
      total: overview.value.message_gate.today_count,
      blocked: overview.value.message_gate.block_count
    },
    {
      key: "action" as const,
      title: "动作门",
      status: overview.value.action_gate.status,
      total: overview.value.action_gate.today_count,
      blocked: overview.value.action_gate.block_count
    },
    {
      key: "return" as const,
      title: "返回门",
      status: overview.value.return_gate.status,
      total: overview.value.return_gate.today_count,
      blocked: overview.value.return_gate.block_count
    }
  ];
});

const gatePulseMetrics = computed(() => {
  const cards = gateStatusCards.value;
  return cards.map(item => ({
    ...item,
    ratio: item.total ? Math.round((item.blocked / item.total) * 100) : 0
  }));
});

const topAgents = computed(() => {
  return (
    stats.value?.top_agents?.slice(0, 6) ?? [
      { agent_id: "finance-agent", count: 86 },
      { agent_id: "workflow-agent", count: 74 },
      { agent_id: "ops-agent", count: 63 },
      { agent_id: "policy-agent", count: 51 },
      { agent_id: "audit-agent", count: 40 },
      { agent_id: "reasoner-agent", count: 34 }
    ]
  );
});

const regionRanking = computed(() => {
  return [...nationalHeatData.value].sort((a, b) => b.value - a.value).slice(0, 6);
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
    {
      requestId: "REQ-7A29F1",
      gate: "action",
      level: "critical",
      reason: "高危工具调用链被阻断",
      score: 96
    },
    {
      requestId: "REQ-37D9CC",
      gate: "message",
      level: "high",
      reason: "Prompt Injection 诱导被识别",
      score: 88
    },
    {
      requestId: "REQ-95AF20",
      gate: "return",
      level: "high",
      reason: "敏感结果外发风险触发审查",
      score: 81
    },
    {
      requestId: "REQ-6C10B8",
      gate: "action",
      level: "medium",
      reason: "越权访问意图被降级执行",
      score: 67
    },
    {
      requestId: "REQ-5D20AE",
      gate: "message",
      level: "low",
      reason: "普通搜索任务通过上下文校验",
      score: 18
    }
  ];
});

const mapSignals = computed(() => [
  {
    label: "联防节点",
    value: "34",
    note: "全国在线"
  },
  {
    label: "跨域攻击链",
    value: `${stats.value?.attack_chains || 28}`,
    note: "实时追踪"
  },
  {
    label: "审计吞吐",
    value: `${stats.value?.total_events || 1523}`,
    note: "总事件量"
  },
  {
    label: "响应时延",
    value: avgResponseMs.value,
    note: "平均耗时"
  }
]);

const systemSignals = computed(() => [
  {
    label: "SM2 签名链路",
    value: sm2Status.value ? "在线" : "离线",
    active: sm2Status.value
  },
  {
    label: "SM3 摘要校验",
    value: sm3Status.value ? "在线" : "离线",
    active: sm3Status.value
  },
  {
    label: "SM4 加密隧道",
    value: sm4Status.value ? "在线" : "离线",
    active: sm4Status.value
  },
  {
    label: "吊销令牌",
    value: `${revokedTokens.value} 枚`,
    active: revokedTokens.value === 0
  }
]);

const defenseTimeline = computed(() => [
  { time: "00:12", title: "策略基线更新", detail: "新增工具参数敏感词约束 3 条" },
  { time: "04:38", title: "跨域行为聚类", detail: "东北到华东流量关联提升 16%" },
  { time: "09:20", title: "动作门告警", detail: "高风险 shell 调用被自动降级" },
  { time: "14:56", title: "审计链闭环", detail: "关联 5 段攻击路径完成溯源" },
  { time: "18:43", title: "令牌预算校验", detail: "可信调用预算全部通过" }
]);

const footerMarquee = computed(() => {
  const expiryText = keyExpiry.value
    ? `密钥到期时间 ${keyExpiry.value}`
    : "密钥轮换计划正常";
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

function renderTrendChart() {
  if (!trendChartRef.value) return;
  if (!trendChart) {
    trendChart = echarts.init(trendChartRef.value);
  }

  trendChart.setOption({
    animationDuration: 1200,
    grid: {
      left: 34,
      right: 12,
      top: 28,
      bottom: 26
    },
    tooltip: {
      trigger: "axis"
    },
    legend: {
      top: 0,
      right: 0,
      textStyle: {
        color: "#8aa7ee"
      }
    },
    xAxis: {
      type: "category",
      data: trendHours.value,
      axisLine: { lineStyle: { color: "rgba(118, 178, 255, 0.4)" } },
      axisLabel: { color: "#98b6ff" }
    },
    yAxis: {
      type: "value",
      splitLine: { lineStyle: { color: "rgba(52, 84, 144, 0.18)" } },
      axisLabel: { color: "#7e9ce8" }
    },
    series: [
      {
        name: "告警流量",
        type: "line",
        smooth: true,
        symbol: "none",
        lineStyle: {
          width: 3,
          color: "#39d0ff"
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: "rgba(57, 208, 255, 0.45)" },
            { offset: 1, color: "rgba(57, 208, 255, 0.02)" }
          ])
        },
        data: trendAlerts.value
      },
      {
        name: "阻断处置",
        type: "line",
        smooth: true,
        symbol: "none",
        lineStyle: {
          width: 2,
          color: "#8d7dff"
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: "rgba(141, 125, 255, 0.28)" },
            { offset: 1, color: "rgba(141, 125, 255, 0.02)" }
          ])
        },
        data: trendBlocks.value
      }
    ]
  });
}

function renderDecisionChart() {
  if (!decisionChartRef.value) return;
  if (!decisionChart) {
    decisionChart = echarts.init(decisionChartRef.value);
  }

  const messageCounts = overview.value?.message_gate.decision_counts ?? {};
  const actionCounts = overview.value?.action_gate.decision_counts ?? {};
  const returnCounts = overview.value?.return_gate.decision_counts ?? {};

  const allow =
    (messageCounts.Allow || 0) +
      (actionCounts.Allow || 0) +
      (returnCounts.Allow || 0) ||
    420;
  const block =
    (messageCounts.Block || 0) +
      (messageCounts.Deny || 0) +
      (actionCounts.Block || 0) +
      (actionCounts.Deny || 0) +
      (returnCounts.Block || 0) +
      (returnCounts.Deny || 0) ||
    89;
  const degrade =
    (messageCounts.Degrade || 0) +
      (actionCounts.Degrade || 0) +
      (returnCounts.Degrade || 0) ||
    56;
  const approval =
    (messageCounts.HumanApproval || 0) +
      (actionCounts.HumanApproval || 0) +
      (returnCounts.HumanApproval || 0) ||
    22;

  decisionChart.setOption({
    animationDuration: 1200,
    tooltip: {
      trigger: "item"
    },
    legend: {
      bottom: 0,
      textStyle: { color: "#8aa7ee" }
    },
    series: [
      {
        type: "pie",
        radius: ["56%", "78%"],
        center: ["42%", "50%"],
        avoidLabelOverlap: false,
        label: {
          show: false
        },
        itemStyle: {
          borderColor: "rgba(8, 16, 44, 0.96)",
          borderWidth: 4
        },
        data: [
          { name: "允许", value: allow, itemStyle: { color: "#34d4ff" } },
          { name: "阻断", value: block, itemStyle: { color: "#ff5f9f" } },
          { name: "降级", value: degrade, itemStyle: { color: "#6cf7b8" } },
          { name: "人工审批", value: approval, itemStyle: { color: "#8b7bff" } }
        ]
      },
      {
        type: "bar",
        xAxisIndex: 0,
        yAxisIndex: 0,
        data: [allow, block, degrade, approval],
        barWidth: 10,
        itemStyle: {
          borderRadius: 10,
          color: (params: any) =>
            ["#34d4ff", "#ff5f9f", "#6cf7b8", "#8b7bff"][params.dataIndex]
        }
      }
    ],
    grid: {
      left: "58%",
      right: 10,
      top: 26,
      bottom: 34
    },
    xAxis: [
      {
        type: "value",
        show: false,
        max: Math.max(allow, block, degrade, approval) * 1.15
      }
    ],
    yAxis: [
      {
        type: "category",
        inverse: true,
        axisTick: { show: false },
        axisLine: { show: false },
        axisLabel: {
          color: "#a4c2ff"
        },
        data: ["允许", "阻断", "降级", "审批"]
      }
    ]
  });
}

function renderFunnelChart() {
  if (!funnelChartRef.value) return;
  if (!funnelChart) {
    funnelChart = echarts.init(funnelChartRef.value);
  }

  funnelChart.setOption({
    animationDuration: 1200,
    tooltip: {
      trigger: "item"
    },
    series: [
      {
        name: "攻击阶段",
        type: "funnel",
        left: "6%",
        top: 10,
        bottom: 8,
        width: "88%",
        minSize: "35%",
        maxSize: "95%",
        sort: "descending",
        gap: 4,
        label: {
          color: "#e1f4ff",
          formatter: "{b}"
        },
        labelLine: {
          length: 8,
          lineStyle: { color: "#5e8cff" }
        },
        itemStyle: {
          borderColor: "rgba(6, 18, 38, 0.92)",
          borderWidth: 2
        },
        data: stageData.value.map((item, index) => ({
          ...item,
          itemStyle: {
            color: ["#34d4ff", "#5f9fff", "#8b7bff", "#ff7db0", "#ffb36b"][index]
          }
        }))
      }
    ]
  });
}

function renderRadarChart() {
  if (!radarChartRef.value) return;
  if (!radarChart) {
    radarChart = echarts.init(radarChartRef.value);
  }

  radarChart.setOption({
    animationDuration: 1200,
    tooltip: {},
    radar: {
      radius: "64%",
      splitNumber: 4,
      indicator: [
        { name: "授权可信", max: 100 },
        { name: "闸门阻断", max: 100 },
        { name: "审计闭环", max: 100 },
        { name: "链路韧性", max: 100 },
        { name: "沙箱隔离", max: 100 },
        { name: "策略覆盖", max: 100 }
      ],
      axisName: {
        color: "#a7c4ff"
      },
      splitLine: {
        lineStyle: {
          color: "rgba(89, 146, 255, 0.2)"
        }
      },
      splitArea: {
        areaStyle: {
          color: ["rgba(7, 20, 54, 0.18)", "rgba(7, 20, 54, 0.08)"]
        }
      },
      axisLine: {
        lineStyle: {
          color: "rgba(89, 146, 255, 0.22)"
        }
      }
    },
    series: [
      {
        type: "radar",
        symbol: "circle",
        symbolSize: 6,
        itemStyle: {
          color: "#78f5ff"
        },
        areaStyle: {
          color: "rgba(71, 226, 255, 0.18)"
        },
        lineStyle: {
          width: 2,
          color: "#78f5ff"
        },
        data: [
          {
            value: radarValues.value,
            name: "联防健康度"
          }
        ]
      }
    ]
  });
}

function renderHealthChart() {
  if (!healthChartRef.value) return;
  if (!healthChart) {
    healthChart = echarts.init(healthChartRef.value);
  }

  healthChart.setOption({
    animationDuration: 1200,
    tooltip: {
      position: "top",
      formatter: (params: any) => {
        return `${healthServices[params.value[0]]} / ${healthMetrics[params.value[1]]}<br/>健康度 ${params.value[2]}`;
      }
    },
    grid: {
      left: 54,
      right: 10,
      top: 20,
      bottom: 20
    },
    xAxis: {
      type: "category",
      data: healthServices,
      splitArea: { show: false },
      axisLabel: {
        color: "#96b4f8"
      },
      axisLine: {
        lineStyle: {
          color: "rgba(89, 146, 255, 0.22)"
        }
      }
    },
    yAxis: {
      type: "category",
      data: healthMetrics,
      splitArea: { show: false },
      axisLabel: {
        color: "#96b4f8"
      },
      axisLine: {
        lineStyle: {
          color: "rgba(89, 146, 255, 0.22)"
        }
      }
    },
    visualMap: {
      min: 70,
      max: 100,
      show: false,
      inRange: {
        color: ["#173372", "#2357c7", "#2ccfff", "#77ffd8"]
      }
    },
    series: [
      {
        type: "heatmap",
        data: healthMatrix.value,
        label: {
          show: true,
          color: "#e9fbff",
          fontSize: 11,
          formatter: (params: any) => params.value[2]
        },
        itemStyle: {
          borderColor: "rgba(5, 18, 38, 0.8)",
          borderWidth: 1,
          borderRadius: 8
        }
      }
    ]
  });
}

async function renderMapChart() {
  if (!mapChartRef.value) return;
  if (!mapChart) {
    mapChart = echarts.init(mapChartRef.value);
  }

  await loadChinaMap();

  mapChart.setOption({
    animationDuration: 1600,
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
        areaColor: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: "#123a84" },
          { offset: 0.6, color: "#0b1f4b" },
          { offset: 1, color: "#08122c" }
        ]),
        borderColor: "#2ecbff",
        borderWidth: 1,
        shadowBlur: 24,
        shadowColor: "rgba(46, 203, 255, 0.35)"
      },
      emphasis: {
        itemStyle: {
          areaColor: "#1b5cb0"
        },
        label: {
          color: "#fff"
        }
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
        name: "全国威胁热度",
        type: "map",
        map: "china",
        geoIndex: 0,
        data: nationalHeatData.value
      },
      {
        name: "威胁节点",
        type: "effectScatter",
        coordinateSystem: "geo",
        rippleEffect: {
          scale: 5,
          brushType: "stroke"
        },
        symbolSize: (val: number[]) => Math.max(8, val[2] / 8),
        itemStyle: {
          color: "#75f2ff",
          shadowBlur: 20,
          shadowColor: "#75f2ff"
        },
        data: cityScatterData.value
      },
      {
        name: "跨域攻击链",
        type: "lines",
        coordinateSystem: "geo",
        zlevel: 2,
        effect: {
          show: true,
          period: 4,
          trailLength: 0.3,
          symbol: "arrow",
          symbolSize: 7,
          color: "#ff79bd"
        },
        lineStyle: {
          color: "#8f6bff",
          width: 1.5,
          opacity: 0.68,
          curveness: 0.28
        },
        data: attackLinks.value
      }
    ]
  });
}

function renderAllCharts() {
  renderTrendChart();
  renderDecisionChart();
  renderFunnelChart();
  renderRadarChart();
  renderHealthChart();
  void renderMapChart();
}

function rotateVisualData() {
  trendHours.value = [...trendHours.value.slice(1), trendHours.value[0]];
  trendAlerts.value = [...trendAlerts.value.slice(1), Math.max(24, Math.min(82, trendAlerts.value[0] + 6 - Math.floor(Math.random() * 12)))];
  trendBlocks.value = [...trendBlocks.value.slice(1), Math.max(12, Math.min(62, trendBlocks.value[0] + 4 - Math.floor(Math.random() * 8)))];

  stageData.value = stageData.value.map((item, index) => ({
    ...item,
    value: Math.max(18, item.value + (index % 2 === 0 ? 5 : -4))
  }));

  radarValues.value = radarValues.value.map(value =>
    Math.max(72, Math.min(99, value + (Math.random() > 0.5 ? 2 : -2)))
  );

  nationalHeatData.value = nationalHeatData.value.map((item, index) => ({
    ...item,
    value: Math.max(42, Math.min(99, item.value + ((index % 3) - 1) * 2))
  }));

  cityScatterData.value = cityScatterData.value.map(item => ({
    ...item,
    value: [item.value[0], item.value[1], Math.max(48, Math.min(99, item.value[2] + (Math.random() > 0.5 ? 3 : -2)))]
  }));

  healthMatrix.value = healthMatrix.value.map(([x, y, v], index) => [
    x,
    y,
    Math.max(72, Math.min(99, v + ((index % 4) - 1)))
  ]);

  rotatingSignals.value = [...rotatingSignals.value.slice(1), rotatingSignals.value[0]];
  renderAllCharts();
}

function resizeCharts() {
  mapChart?.resize();
  trendChart?.resize();
  decisionChart?.resize();
  funnelChart?.resize();
  radarChart?.resize();
  healthChart?.resize();
}

async function initializeScreen() {
  await Promise.all([loadOverview(), loadStats(), refresh()]);
  await nextTick();
  renderAllCharts();
}

onMounted(async () => {
  await initializeScreen();
  startGatePolling(6000);
  startAuditPolling(6000);
  animationTimer = setInterval(rotateVisualData, 4500);
  window.addEventListener("resize", resizeCharts);
});

onUnmounted(() => {
  stopGatePolling();
  stopAuditPolling();
  if (animationTimer) {
    clearInterval(animationTimer);
    animationTimer = null;
  }
  window.removeEventListener("resize", resizeCharts);
  mapChart?.dispose();
  trendChart?.dispose();
  decisionChart?.dispose();
  funnelChart?.dispose();
  radarChart?.dispose();
  healthChart?.dispose();
  mapChart = null;
  trendChart = null;
  decisionChart = null;
  funnelChart = null;
  radarChart = null;
  healthChart = null;
});
</script>

<template>
  <div class="landing-screen">
    <div class="bg-grid"></div>
    <div class="bg-glow glow-left"></div>
    <div class="bg-glow glow-right"></div>
    <div class="scan-line"></div>

    <div
      v-for="(particle, index) in particles"
      :key="index"
      class="particle"
      :style="{
        top: particle.top,
        left: particle.left,
        width: particle.size,
        height: particle.size,
        animationDelay: particle.delay,
        animationDuration: particle.duration
      }"
    ></div>

    <header class="topbar">
      <div class="brand">
        <div class="brand-badge">AG</div>
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
        <button class="enter-button" @click="router.push('/login')">
          进入注册 / 登录
        </button>
      </div>
    </header>

    <main class="screen-layout">
      <section class="panel panel-left">
        <div class="panel-card metrics-card">
          <div class="card-title">核心战情指标</div>
          <div class="metrics-grid metrics-grid-large">
            <div
              v-for="item in systemMetrics"
              :key="item.label"
              class="metric-box"
              :class="item.accent"
            >
              <p>{{ item.label }}</p>
              <strong>{{ item.value }}</strong>
              <span>{{ item.unit }}</span>
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

        <div class="panel-card chart-card">
          <div class="card-title">攻击阶段漏斗</div>
          <div ref="funnelChartRef" class="chart-box chart-sm"></div>
        </div>

        <!-- 地区热度排行 已移动到联防时间线旁 -->
      </section>

      <section class="panel panel-center">
        <div class="hero-panel">
            <div class="hero-caption">
            <span>全国联防热力图</span>
            <strong>Threat Mapping Matrix</strong>
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

          <div class="panel-card chart-card waveform-card">
            <div class="card-title">24H 威胁波形</div>
            <div ref="trendChartRef" class="chart-box chart-sm"></div>
          </div>

          <div class="panel-card rank-card">
            <div class="card-title">地区热度排行</div>
            <div class="region-rank">
              <div v-for="(region, index) in regionRanking" :key="region.name" class="region-row">
                <span class="region-index">{{ index + 1 }}</span>
                <span class="region-name">{{ region.name }}</span>
                <div class="region-bar">
                  <div class="region-bar-fill" :style="{ width: `${region.value}%` }"></div>
                </div>
                <strong>{{ region.value }}</strong>
              </div>
            </div>
          </div>

          <div class="panel-card pulse-card">
            <div class="card-title">Gate 脉冲矩阵</div>
            <div class="pulse-list">
              <div v-for="item in gatePulseMetrics" :key="item.key" class="pulse-item">
                <div class="pulse-head">
                  <span>{{ item.title }}</span>
                  <strong>{{ item.ratio }}%</strong>
                </div>
                <div class="pulse-track">
                  <div class="pulse-fill" :style="{ width: `${item.ratio}%` }"></div>
                </div>
                <small>{{ item.blocked }} / {{ item.total }} 次命中</small>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="panel panel-right">
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

        <div class="panel-card chart-card">
          <div class="card-title">决策分布矩阵</div>
          <div ref="decisionChartRef" class="chart-box chart-md"></div>
        </div>

        <div class="panel-card chart-card compact-card">
          <div class="card-title">联防健康雷达</div>
          <div ref="radarChartRef" class="chart-box chart-compact radar-tall"></div>
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
  --panel-border: rgba(75, 173, 255, 0.28);
  --text-main: #dff4ff;
  --text-soft: #85a8eb;
  --cyan: #3edcff;
  --blue: #5d8bff;
  --magenta: #ff62b2;
  --emerald: #69ffc8;
  position: relative;
  height: 100vh;
  overflow: hidden;
  background:
    radial-gradient(circle at 15% 20%, rgba(44, 150, 255, 0.24), transparent 24%),
    radial-gradient(circle at 80% 12%, rgba(138, 101, 255, 0.22), transparent 26%),
    radial-gradient(circle at 50% 80%, rgba(0, 255, 205, 0.09), transparent 30%),
    linear-gradient(180deg, #040816 0%, #07102c 55%, #020611 100%);
  color: var(--text-main);
}

.bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(75, 122, 255, 0.09) 1px, transparent 1px),
    linear-gradient(90deg, rgba(75, 122, 255, 0.09) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: radial-gradient(circle at center, black 48%, transparent 100%);
}

.bg-glow {
  position: absolute;
  width: 32vw;
  height: 32vw;
  filter: blur(72px);
  opacity: 0.3;
}

.glow-left {
  top: -8vw;
  left: -6vw;
  background: rgba(61, 165, 255, 0.45);
  animation: pulseGlow 8s ease-in-out infinite;
}

.glow-right {
  right: -10vw;
  bottom: -12vw;
  background: rgba(137, 89, 255, 0.4);
  animation: pulseGlow 10s ease-in-out infinite reverse;
}

.scan-line {
  position: absolute;
  inset: -20% 0 auto;
  height: 24%;
  background: linear-gradient(180deg, transparent, rgba(76, 222, 255, 0.12), transparent);
  animation: scan 9s linear infinite;
}

.particle {
  position: absolute;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(117, 246, 255, 0.95), rgba(117, 246, 255, 0.05));
  box-shadow: 0 0 18px rgba(117, 246, 255, 0.8);
  animation-name: floatParticle;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
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
  border: 1px solid rgba(96, 232, 255, 0.36);
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(44, 147, 255, 0.28), rgba(103, 244, 255, 0.08));
  box-shadow: inset 0 0 12px rgba(103, 244, 255, 0.12), 0 0 20px rgba(62, 220, 255, 0.12);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.08em;
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
  color: var(--text-soft);
  font-size: 11px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.headline-box {
  padding: 10px 24px;
  border: 1px solid rgba(61, 188, 255, 0.22);
  border-radius: 999px;
  background: rgba(6, 14, 38, 0.72);
  text-align: center;
  box-shadow: inset 0 0 24px rgba(65, 158, 255, 0.12);
}

.headline-box h1 {
  margin: 6px 0 0;
  font-size: clamp(16px, 1.2vw, 22px);
  font-weight: 700;
  letter-spacing: 0.12em;
}

.enter-button {
  position: relative;
  overflow: hidden;
  padding: 12px 20px;
  border: 1px solid rgba(111, 223, 255, 0.48);
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(27, 102, 210, 0.28), rgba(67, 232, 255, 0.18));
  color: #ecfbff;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.08em;
  cursor: pointer;
  transition:
    transform 0.25s ease,
    box-shadow 0.25s ease,
    border-color 0.25s ease;
  box-shadow: 0 0 24px rgba(62, 220, 255, 0.18);
}

.enter-button:hover {
  transform: translateY(-1px);
  border-color: rgba(145, 242, 255, 0.9);
  box-shadow: 0 0 34px rgba(62, 220, 255, 0.32);
}

  .screen-layout {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: 13vw minmax(760px, 1fr) 13vw;
  gap: 10px;
  height: calc(100vh - 92px);
  padding: 0 14px 12px;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}

.panel-left,
.panel-center,
.panel-right {
  min-height: 0;
}

.panel-card,
.hero-panel {
  position: relative;
  border: 1px solid var(--panel-border);
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(10, 22, 54, 0.82), rgba(3, 10, 28, 0.72));
  box-shadow:
    inset 0 0 0 1px rgba(104, 192, 255, 0.08),
    0 14px 32px rgba(2, 8, 22, 0.34);
  backdrop-filter: blur(14px);
}

.panel-card::before,
.hero-panel::before {
  content: "";
  position: absolute;
  inset: 1px;
  border-radius: inherit;
  background:
    linear-gradient(140deg, rgba(117, 246, 255, 0.1), transparent 24%, transparent 70%, rgba(141, 125, 255, 0.08)),
    radial-gradient(circle at top right, rgba(77, 209, 255, 0.08), transparent 30%);
  pointer-events: none;
}

.panel-card {
  padding: 12px;
}

.panel-right {
  padding-right: 6px;
}

.panel-right .panel-card {
  min-height: 200px;
}

.panel-right .chart-card {
  min-height: 220px;
}

.card-title {
  position: relative;
  margin-bottom: 8px;
  padding-left: 12px;
  color: #ecf9ff;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.card-title::before {
  content: "";
  position: absolute;
  left: 0;
  top: 2px;
  bottom: 2px;
  width: 4px;
  border-radius: 999px;
  background: linear-gradient(180deg, var(--cyan), var(--blue));
  box-shadow: 0 0 12px rgba(62, 220, 255, 0.72);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.metrics-grid-large {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.metric-box {
  padding: 6px 8px;
  border: 1px solid rgba(97, 154, 255, 0.12);
  border-radius: 14px;
  background: rgba(6, 16, 42, 0.72);
}

.metric-box p {
  font-size: 11px;
  margin-bottom: 4px;
}

.metric-box strong {
  display: inline-block;
  margin-top: 2px;
  margin-right: 4px;
  font-size: 18px;
  line-height: 1.05;
}

.metric-box p,
.signal-item span,
.token-summary span,
.map-kpis span,
.gate-item span,
.alert-item small,
.agent-name,
.region-name,
.overlay-card span,
.timeline-time,
.pulse-item small {
  color: var(--text-soft);
  font-size: 12px;
}

.metric-box strong {
  display: inline-block;
  margin-top: 4px;
  margin-right: 6px;
  font-size: 20px;
  line-height: 1.05;
}

.metric-box.cyan strong {
  color: var(--cyan);
}

.metric-box.blue strong {
  color: #79a6ff;
}

.metric-box.emerald strong {
  color: var(--emerald);
}

.metric-box.magenta strong {
  color: #ff88d4;
}

.signal-list,
.region-rank,
.timeline-list,
.pulse-list {
  display: grid;
  gap: 10px;
}

.signal-item,
.gate-item,
.agent-item {
  display: grid;
  align-items: center;
}

.signal-item {
  grid-template-columns: auto 1fr auto;
  gap: 6px;
  padding: 5px 8px;
  border: 1px solid rgba(100, 158, 255, 0.12);
  border-radius: 12px;
  background: rgba(4, 14, 36, 0.72);
}

.signal-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: rgba(255, 111, 163, 0.85);
  box-shadow: 0 0 10px rgba(255, 111, 163, 0.35);
}

.signal-dot.active {
  background: var(--emerald);
  box-shadow: 0 0 14px rgba(105, 255, 200, 0.7);
}

.token-summary,
.map-kpis {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.token-summary {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.token-summary div,
.map-kpis div,
.overlay-card {
  padding: 8px 10px;
  border-radius: 14px;
  background: rgba(7, 17, 44, 0.78);
  border: 1px solid rgba(90, 156, 255, 0.12);
}

.token-summary strong,
.map-kpis strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--cyan);
}

.token-summary strong + small,
.map-kpis small {
  margin-top: 4px;
  display: inline-block;
  color: #7fa3ef;
  font-size: 12px;
}

.chart-card {
  min-height: 0;
}

.panel-left .chart-card {
  min-height: 230px;
}

.chart-box {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.chart-sm {
  height: 220px;
}

.chart-md {
  height: 260px;
}

.chart-compact {
  height: 170px;
}

.matrix-chart {
  height: 200px;
}

.hero-panel {
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 12px;
  padding: 16px 16px 18px;
  overflow: hidden;
}

.hero-main-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) 180px;
  gap: 10px;
  min-height: 520px;
}

.hero-map-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hero-side-panel {
  display: grid;
  gap: 8px;
}

.hero-rings {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.ring {
  position: absolute;
  inset: 0;
  border: 1px solid rgba(84, 183, 255, 0.16);
  border-radius: 50%;
}

.ring-1 {
  transform: scale(0.58);
  animation: rotateRing 18s linear infinite;
}

.ring-2 {
  transform: scale(0.78);
  border-style: dashed;
  animation: rotateRing 24s linear infinite reverse;
}

.ring-3 {
  transform: scale(1);
  opacity: 0.66;
  animation: pulseRing 6s ease-in-out infinite;
}

.hero-caption {
  position: absolute;
  left: 28px;
  top: 22px;
  z-index: 3;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-caption span {
  color: var(--text-soft);
  font-size: 13px;
  letter-spacing: 0.16em;
}

.hero-caption strong {
  font-size: 20px;
  letter-spacing: 0.08em;
}

.hero-insight-rail {
  position: relative;
  z-index: 3;
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
  margin: 0;
}

.hero-insight-card {
  min-height: 64px;
  padding: 8px 10px;
  border: 1px solid rgba(84, 183, 255, 0.16);
  border-radius: 16px;
  background: linear-gradient(180deg, rgba(5, 16, 42, 0.88), rgba(4, 12, 30, 0.9));
  box-shadow:
    0 10px 18px rgba(2, 8, 22, 0.28),
    inset 0 0 0 1px rgba(98, 188, 255, 0.08);
}

.hero-insight-card span {
  display: block;
  margin-bottom: 6px;
  color: #8fb3ff;
  font-size: 11px;
}

.hero-insight-card strong {
  display: block;
  line-height: 1.4;
  font-size: 13px;
  color: #edfaff;
}

.map-chart {
  position: relative;
  z-index: 2;
  min-height: 540px;
  height: 100%;
  border-radius: 24px;
  overflow: hidden;
  background: rgba(3, 10, 28, 0.78);
}

.map-kpis {
  position: relative;
  z-index: 2;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.map-kpis-dense strong {
  font-size: 20px;
}

.center-bottom-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  align-items: start;
}

.bottom-right-wrapper {
  display: grid;
  gap: 10px;
}

.right-bottom-column {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.rank-card .region-rank {
  max-height: 180px;
  overflow: auto;
  padding-right: 6px;
}

.pulse-list {
  max-height: 180px;
  overflow: auto;
}

.center-bottom-grid .panel-card {
  min-height: 220px;
  overflow: hidden;
}

.center-bottom-grid .timeline-card {
  min-height: 240px;
}

.center-bottom-grid .waveform-card {
  min-height: 220px;
}

.pulse-card .pulse-list,
.rank-card .region-rank {
  max-height: 170px;
}

.timeline-list,
.pulse-list {
  max-height: 200px;
  overflow-y: auto;
}

.timeline-list::-webkit-scrollbar,
.pulse-list::-webkit-scrollbar {
  width: 6px;
}

.timeline-list::-webkit-scrollbar-thumb,
.pulse-list::-webkit-scrollbar-thumb {
  background: rgba(62, 220, 255, 0.25);
  border-radius: 999px;
}

.timeline-list {
  padding-right: 8px;
}

.compact-card {
  min-height: 0;
}

.radar-tall {
  height: 178px;
}

.timeline-item {
  display: grid;
  grid-template-columns: 56px 1fr;
  gap: 10px;
  align-items: start;
}

.timeline-time {
  padding-top: 2px;
  font-size: 12px;
}

.timeline-body {
  position: relative;
  padding-left: 14px;
}

.timeline-body::before {
  content: "";
  position: absolute;
  left: 0;
  top: 4px;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #3edcff;
  box-shadow: 0 0 14px rgba(62, 220, 255, 0.85);
}

.timeline-body::after {
  content: "";
  position: absolute;
  left: 3px;
  top: 16px;
  bottom: -18px;
  width: 1px;
  background: linear-gradient(180deg, rgba(62, 220, 255, 0.45), transparent);
}

.timeline-item:last-child .timeline-body::after {
  display: none;
}

.timeline-body strong {
  display: block;
  margin-bottom: 4px;
}

.timeline-body p {
  margin: 0;
  color: #91b2f7;
  line-height: 1.5;
}

.pulse-item {
  padding: 10px 12px;
  border: 1px solid rgba(95, 154, 255, 0.14);
  border-radius: 14px;
  background: rgba(3, 12, 30, 0.7);
}

.pulse-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.pulse-head strong {
  color: #78f5ff;
}

.pulse-track,
.region-bar,
.agent-bar {
  position: relative;
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(71, 109, 202, 0.22);
}

.pulse-fill,
.region-bar-fill,
.agent-bar-fill {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: inherit;
  box-shadow: 0 0 16px rgba(62, 220, 255, 0.4);
}

.pulse-fill {
  background: linear-gradient(90deg, #35d9ff, #9b7bff);
}

.region-rank,
.gate-list,
.alert-list,
.agent-rank {
  display: grid;
  gap: 10px;
}

.region-row {
  display: grid;
  grid-template-columns: 26px 54px 1fr 34px;
  gap: 10px;
  align-items: center;
}

.region-index,
.rank-index {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: linear-gradient(135deg, rgba(61, 220, 255, 0.25), rgba(141, 125, 255, 0.25));
  color: #effbff;
  font-size: 12px;
  font-weight: 700;
}

.region-bar-fill {
  background: linear-gradient(90deg, #2ecfff, #7d8aff);
}

.gate-item {
  grid-template-columns: 1fr auto;
  padding: 10px 12px;
  border: 1px solid rgba(110, 172, 255, 0.14);
  border-radius: 18px;
  background: rgba(3, 14, 34, 0.74);
}

.gate-item p,
.alert-item p {
  margin: 0 0 6px;
}

.gate-values {
  text-align: right;
}

.gate-values strong {
  display: block;
  font-size: 24px;
  color: var(--cyan);
}

.alert-item {
  padding: 14px;
  border: 1px solid rgba(102, 158, 255, 0.14);
  border-radius: 18px;
  background: rgba(3, 12, 30, 0.74);
}

.alert-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.risk-tag {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.risk-tag.critical {
  background: rgba(255, 95, 159, 0.16);
  color: #ff8ac5;
}

.risk-tag.high {
  background: rgba(255, 124, 95, 0.16);
  color: #ffaf93;
}

.risk-tag.medium {
  background: rgba(92, 138, 255, 0.16);
  color: #98b6ff;
}

.risk-tag.low {
  background: rgba(95, 255, 185, 0.16);
  color: #7df2c5;
}

.agent-item {
  grid-template-columns: 28px minmax(0, 1fr) minmax(74px, 1fr) auto;
  gap: 10px;
}

.rank-index {
  width: 28px;
  height: 28px;
  font-size: 13px;
}

.agent-bar-fill {
  background: linear-gradient(90deg, var(--cyan), #7a86ff);
}

.ticker {
  position: absolute;
  right: 18px;
  bottom: 16px;
  left: 18px;
  z-index: 2;
  overflow: hidden;
  border: 1px solid rgba(94, 160, 255, 0.16);
  border-radius: 999px;
  background: rgba(4, 11, 28, 0.88);
}

.ticker-track {
  padding: 10px 0;
  color: #b9d2ff;
  white-space: nowrap;
  animation: tickerMove 24s linear infinite;
}

@keyframes pulseGlow {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.24;
  }
  50% {
    transform: scale(1.18);
    opacity: 0.38;
  }
}

@keyframes scan {
  0% {
    transform: translateY(-20vh);
  }
  100% {
    transform: translateY(120vh);
  }
}

@keyframes floatParticle {
  0%,
  100% {
    transform: translate3d(0, 0, 0) scale(0.92);
    opacity: 0.2;
  }
  50% {
    transform: translate3d(18px, -28px, 0) scale(1.2);
    opacity: 0.95;
  }
}

@keyframes rotateRing {
  from {
    transform: translateZ(0) scale(var(--scale, 1)) rotate(0deg);
  }
  to {
    transform: translateZ(0) scale(var(--scale, 1)) rotate(360deg);
  }
}

.ring-1 {
  --scale: 0.58;
}

.ring-2 {
  --scale: 0.78;
}

@keyframes pulseRing {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.48;
  }
  50% {
    transform: scale(1.05);
    opacity: 0.88;
  }
}

@keyframes tickerMove {
  0% {
    transform: translateX(100%);
  }
  100% {
    transform: translateX(-100%);
  }
}

@media (max-width: 1600px) {
  .screen-layout {
    grid-template-columns: 20vw minmax(420px, 1fr) 20vw;
  }

  .hero-floating-right {
    right: 26px;
    top: 146px;
  }

  .map-side-overlay {
    width: 230px;
  }
}

@media (max-width: 1440px) {
  .hero-floating,
  .map-side-overlay,
  .hero-floating-right {
    display: none;
  }

  .mini-chart-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1280px) {
  .landing-screen {
    overflow-y: auto;
  }

  .topbar {
    grid-template-columns: 1fr;
    gap: 16px;
    justify-items: center;
    text-align: center;
  }

  .screen-layout {
    grid-template-columns: 1fr;
    height: auto;
    padding-bottom: 100px;
  }

  .center-bottom-grid {
    grid-template-columns: 1fr;
  }

  .mini-chart-grid {
    grid-template-columns: 1fr;
  }

  .map-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .panel-left,
  .panel-right {
    order: 2;
  }

  .panel-center {
    order: 1;
  }

  .panel-right {
    overflow: visible;
    padding-right: 0;
  }

  .map-chart {
    min-height: 420px;
  }
}
</style>
