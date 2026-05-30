<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted, computed, nextTick } from "vue";
import * as echarts from "echarts";

interface DisposalStats {
  totalIntercepts: number;
  totalAllowed: number;
  messageGateBlocks: number;
  actionGateBlocks: number;
  returnGateBlocks: number;
  authDenials: number;
  sandboxIsolations: number;
}

defineOptions({ name: "DisposalFlowGraph" });

const props = defineProps<{
  stats: DisposalStats;
}>();

const chartRef = ref<HTMLElement>();
let chart: echarts.ECharts | null = null;
let isDisposed = false;
const containerWidth = ref(0);
let resizeHandler: (() => void) | null = null;

const flowData = computed(() => {
  const total = props.stats.totalIntercepts + props.stats.totalAllowed;
  const width = containerWidth.value || 800;
  const centerY = 150;
  const nodeSpacing = Math.min(120, width / 8);
  const startX = 60;

  const nodes = [
    {
      id: "input",
      name: "请求输入\nInput",
      x: startX,
      y: centerY,
      symbol: "circle",
      symbolSize: 50,
      itemStyle: { color: "#3b82f6" },
      value: total
    },
    {
      id: "message-gate",
      name: "消息闸门\nMessage Gate",
      x: startX + nodeSpacing,
      y: centerY - 60,
      symbol: "circle",
      symbolSize: 55,
      itemStyle: { color: props.stats.messageGateBlocks > 0 ? "#ef4444" : "#22c55e" },
      value: props.stats.messageGateBlocks,
      label: {
        formatter: `消息闸门\n${props.stats.messageGateBlocks > 0 ? props.stats.messageGateBlocks + ' 拦截' : '通过'}`
      }
    },
    {
      id: "auth",
      name: "授权校验\nAuthorization",
      x: startX + nodeSpacing * 2,
      y: centerY,
      symbol: "circle",
      symbolSize: 55,
      itemStyle: { color: props.stats.authDenials > 0 ? "#ef4444" : "#22c55e" },
      value: props.stats.authDenials,
      label: {
        formatter: `授权校验\n${props.stats.authDenials > 0 ? props.stats.authDenials + ' 拒绝' : '通过'}`
      }
    },
    {
      id: "action-gate",
      name: "动作闸门\nAction Gate",
      x: startX + nodeSpacing * 3,
      y: centerY - 60,
      symbol: "circle",
      symbolSize: 55,
      itemStyle: { color: props.stats.actionGateBlocks > 0 ? "#ef4444" : "#22c55e" },
      value: props.stats.actionGateBlocks,
      label: {
        formatter: `动作闸门\n${props.stats.actionGateBlocks > 0 ? props.stats.actionGateBlocks + ' 拦截' : '通过'}`
      }
    },
    {
      id: "sandbox",
      name: "沙箱隔离\nSandbox",
      x: startX + nodeSpacing * 3,
      y: centerY + 60,
      symbol: "circle",
      symbolSize: 55,
      itemStyle: { color: props.stats.sandboxIsolations > 0 ? "#a855f7" : "#94a3b8" },
      value: props.stats.sandboxIsolations,
      label: {
        formatter: `沙箱隔离\n${props.stats.sandboxIsolations > 0 ? props.stats.sandboxIsolations + ' 隔离' : '无'}`
      }
    },
    {
      id: "return-gate",
      name: "返回闸门\nReturn Gate",
      x: startX + nodeSpacing * 4,
      y: centerY,
      symbol: "circle",
      symbolSize: 55,
      itemStyle: { color: props.stats.returnGateBlocks > 0 ? "#ef4444" : "#22c55e" },
      value: props.stats.returnGateBlocks,
      label: {
        formatter: `返回闸门\n${props.stats.returnGateBlocks > 0 ? props.stats.returnGateBlocks + ' 拦截' : '通过'}`
      }
    },
    {
      id: "output",
      name: "响应输出\nOutput",
      x: startX + nodeSpacing * 5,
      y: centerY,
      symbol: "circle",
      symbolSize: 50,
      itemStyle: { color: "#22c55e" },
      value: props.stats.totalAllowed
    }
  ];

  const links = [
    {
      source: "input",
      target: "message-gate",
      lineStyle: { color: "#3b82f6", width: 2 }
    },
    {
      source: "message-gate",
      target: "auth",
      lineStyle: { color: props.stats.messageGateBlocks > 0 ? "#ef4444" : "#22c55e", width: 2 }
    },
    {
      source: "auth",
      target: "action-gate",
      lineStyle: { color: props.stats.authDenials > 0 ? "#ef4444" : "#22c55e", width: 2 }
    },
    {
      source: "auth",
      target: "sandbox",
      lineStyle: { color: "#a855f7", width: 2, type: "dashed" }
    },
    {
      source: "action-gate",
      target: "return-gate",
      lineStyle: { color: props.stats.actionGateBlocks > 0 ? "#ef4444" : "#22c55e", width: 2 }
    },
    {
      source: "sandbox",
      target: "return-gate",
      lineStyle: { color: "#a855f7", width: 2, type: "dashed" }
    },
    {
      source: "return-gate",
      target: "output",
      lineStyle: { color: props.stats.returnGateBlocks > 0 ? "#ef4444" : "#22c55e", width: 2 }
    }
  ];

  // 添加阻断节点（如果有拦截）
  const blockNodes = [];
  const blockLinks = [];
  
  if (props.stats.messageGateBlocks > 0) {
    blockNodes.push({
      id: "block-message",
      name: "已阻断",
      x: startX + nodeSpacing,
      y: centerY - 120,
      symbol: "circle",
      symbolSize: 40,
      itemStyle: { color: "#ef4444" }
    });
    blockLinks.push({
      source: "message-gate",
      target: "block-message",
      lineStyle: { color: "#ef4444", width: 3, type: "dashed" }
    });
  }

  if (props.stats.authDenials > 0) {
    blockNodes.push({
      id: "block-auth",
      name: "已拒绝",
      x: startX + nodeSpacing * 2,
      y: centerY - 60,
      symbol: "circle",
      symbolSize: 40,
      itemStyle: { color: "#ef4444" }
    });
    blockLinks.push({
      source: "auth",
      target: "block-auth",
      lineStyle: { color: "#ef4444", width: 3, type: "dashed" }
    });
  }

  if (props.stats.actionGateBlocks > 0) {
    blockNodes.push({
      id: "block-action",
      name: "已阻断",
      x: startX + nodeSpacing * 3,
      y: centerY - 120,
      symbol: "circle",
      symbolSize: 40,
      itemStyle: { color: "#ef4444" }
    });
    blockLinks.push({
      source: "action-gate",
      target: "block-action",
      lineStyle: { color: "#ef4444", width: 3, type: "dashed" }
    });
  }

  if (props.stats.returnGateBlocks > 0) {
    blockNodes.push({
      id: "block-return",
      name: "已阻断",
      x: startX + nodeSpacing * 4,
      y: centerY - 60,
      symbol: "circle",
      symbolSize: 40,
      itemStyle: { color: "#ef4444" }
    });
    blockLinks.push({
      source: "return-gate",
      target: "block-return",
      lineStyle: { color: "#ef4444", width: 3, type: "dashed" }
    });
  }

  return {
    nodes: [...nodes, ...blockNodes],
    links: [...links, ...blockLinks]
  };
});

function initChart() {
  if (!chartRef.value || isDisposed) return;
  
  // 获取容器实际宽度
  const rect = chartRef.value.getBoundingClientRect();
  containerWidth.value = rect.width;
  
  if (!chart) {
    chart = echarts.init(chartRef.value, undefined, { renderer: 'canvas' });
  }
  
  renderChart();
}

function renderChart() {
  if (!chart || isDisposed || chart.isDisposed() || containerWidth.value === 0) return;

  const { nodes, links } = flowData.value;

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: "item",
      formatter: (params: any) => {
        if (params.dataType === "node") {
          const value = params.data.value;
          const name = params.data.name?.split('\n')[0];
          if (value !== undefined) {
            return `<div style="padding: 8px;"><div style="font-weight: bold;">${name}</div><div>数量: ${value}</div></div>`;
          }
          return name;
        }
        return "";
      }
    },
    animationDuration: 800,
    animationEasing: "cubicOut",
    series: [
      {
        type: "graph",
        layout: "none", // 使用自定义布局
        data: nodes,
        links: links,
        roam: true, // 允许缩放和平移
        draggable: true, // 允许拖动节点
        focusNodeAdjacency: true,
        label: {
          show: true,
          fontSize: 11,
          position: "bottom"
        },
        emphasis: {
          focus: "adjacency",
          lineStyle: {
            width: 4
          }
        },
        lineStyle: {
          opacity: 0.8,
          curveness: 0.1
        },
        edgeSymbol: ["none", "arrow"],
        edgeSymbolSize: [0, 12],
        // 设置画布范围
        left: 20,
        right: 20,
        top: 20,
        bottom: 20
      }
    ]
  };

  chart.setOption(option, true);
}

// 监听数据变化
watch(() => props.stats, (newVal, oldVal) => {
  if (isDisposed) return;
  nextTick(() => {
    initChart();
  });
}, { deep: true });

// 监听容器大小变化
let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  nextTick(() => {
    initChart();
    
    // 使用 ResizeObserver 监听容器大小变化，只调用 resize 不重新渲染
    if (chartRef.value && window.ResizeObserver) {
      resizeObserver = new ResizeObserver(() => {
        if (chart && !chart.isDisposed()) chart.resize();
      });
      resizeObserver.observe(chartRef.value);
    }
  });
  
  resizeHandler = () => {
    if (chart && !chart.isDisposed()) chart.resize();
  };
  window.addEventListener("resize", resizeHandler);
});

onUnmounted(() => {
  isDisposed = true;
  if (resizeObserver && chartRef.value) {
    resizeObserver.unobserve(chartRef.value);
    resizeObserver.disconnect();
    resizeObserver = null;
  }
  if (resizeHandler) {
    window.removeEventListener("resize", resizeHandler);
    resizeHandler = null;
  }
  if (chart) {
    chart.dispose();
    chart = null;
  }
});
</script>

<template>
  <div class="disposal-flow-graph">
    <div ref="chartRef" class="chart-container"></div>
    <div class="flow-legend">
      <div class="legend-item">
        <span class="dot" style="background: #22c55e;"></span>
        <span>正常通过</span>
      </div>
      <div class="legend-item">
        <span class="dot" style="background: #ef4444;"></span>
        <span>拦截/拒绝</span>
      </div>
      <div class="legend-item">
        <span class="dot" style="background: #a855f7;"></span>
        <span>沙箱隔离</span>
      </div>
      <div class="legend-item">
        <span class="line" style="border-color: #3b82f6;"></span>
        <span>正常流程</span>
      </div>
      <div class="legend-item">
        <span class="line dashed" style="border-color: #ef4444;"></span>
        <span>阻断流程</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.disposal-flow-graph {
  width: 100%;
}

.chart-container {
  width: 100% !important;
  min-width: 100%;
  height: 320px;
  min-height: 320px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: linear-gradient(135deg, #f8fafc 0%, #ffffff 100%);
}

.flow-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 12px;
  padding: 8px;
  background: #f8fafc;
  border-radius: 6px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #666;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.line {
  width: 20px;
  height: 0;
  border-top: 2px solid;
}

.line.dashed {
  border-top-style: dashed;
}
</style>
