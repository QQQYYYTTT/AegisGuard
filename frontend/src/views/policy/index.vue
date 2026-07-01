<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import StatCard from "@/components/common/StatCard.vue";
import { usePolicyStoreHook } from "@/store/modules/policy";
import { ElMessage, ElMessageBox } from "element-plus";
import type { PolicyRule, CreateRulePayload } from "@/api/policy";
import ArrowUp from "~icons/ep/arrow-up-bold";
import ArrowDown from "~icons/ep/arrow-down-bold";

defineOptions({ name: "PolicyIndex" });

const policyStore = usePolicyStoreHook();

const activeTab = ref("rules");
const dialogVisible = ref(false);
const editingRule = ref<Partial<PolicyRule> & { _edit?: boolean }>({});
const isEditMode = ref(false);

const gateTypeOptions = [
  { label: "消息闸门 / Message Gate", value: "message" },
  { label: "动作闸门 / Action Gate", value: "action" },
  { label: "返回闸门 / Return Gate", value: "return" },
];

const actionOptions = [
  { label: "放行 / Allow", value: "Allow" },
  { label: "拦截 / Block", value: "Block" },
  { label: "降级 / Degrade", value: "Degrade" },
  { label: "拒绝 / Deny", value: "Deny" },
  { label: "人工审批 / HumanApproval", value: "HumanApproval" },
];

const ruleStats = computed(() => {
  const rules = policyStore.rules;
  return {
    total: rules.length,
    enabled: rules.filter((r) => r.enabled).length,
    blocked: rules.filter((r) => r.action === "Block" || r.action === "Deny").length,
    message: rules.filter((r) => r.gate_type === "message").length,
    action: rules.filter((r) => r.gate_type === "action").length,
    returned: rules.filter((r) => r.gate_type === "return").length,
  };
});

function openCreateDialog() {
  isEditMode.value = false;
  editingRule.value = {
    name: "",
    description: "",
    gate_type: "message",
    condition: "",
    action: "Block",
    priority: policyStore.rules.length + 1,
    enabled: true,
    risk_threshold: 30,
  };
  dialogVisible.value = true;
}

function openEditDialog(rule: PolicyRule) {
  isEditMode.value = true;
  editingRule.value = { ...rule, _edit: true };
  dialogVisible.value = true;
}

async function saveRule() {
  if (!editingRule.value.name?.trim()) {
    ElMessage.warning("请输入规则名称");
    return;
  }
  if (!editingRule.value.condition?.trim() && editingRule.value.action !== "Allow") {
    ElMessage.warning("请填写匹配条件（正则表达式），或选择动作为'放行'");
    return;
  }

  try {
    if (isEditMode.value && editingRule.value.id) {
      await policyStore.updateRule(editingRule.value as PolicyRule);
      ElMessage.success("规则已更新");
    } else {
      await policyStore.createRule(editingRule.value as CreateRulePayload);
      ElMessage.success("规则已创建");
    }
    dialogVisible.value = false;
  } catch (error) {
    ElMessage.error(
      policyStore.lastError ||
        (error instanceof Error ? error.message : "保存规则失败")
    );
  }
}

async function handleDelete(rule: PolicyRule) {
  try {
    await ElMessageBox.confirm(
      `确定删除规则「${rule.name}」？此操作不可撤销。`,
      "确认删除",
      { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" }
    );
    await policyStore.removeRule(rule.id);
    ElMessage.success("规则已删除");
  } catch (error) {
    if (policyStore.lastError) {
      ElMessage.error(policyStore.lastError);
    } else if (error instanceof Error && error.message !== "cancel") {
      ElMessage.error(error.message);
    }
  }
}

async function toggleEnabled(rule: PolicyRule) {
  try {
    const nextEnabled = !rule.enabled;
    await policyStore.toggleRule(rule.id);
    ElMessage.success(nextEnabled ? "规则已启用" : "规则已禁用");
  } catch (error) {
    ElMessage.error(
      policyStore.lastError ||
        (error instanceof Error ? error.message : "切换规则状态失败")
    );
  }
}

async function movePriority(rule: PolicyRule, direction: "up" | "down") {
  const sorted = policyStore.sortedRules;
  const idx = sorted.findIndex((r) => r.id === rule.id);
  if (direction === "up" && idx === 0) return;
  if (direction === "down" && idx === sorted.length - 1) return;

  const swapIdx = direction === "up" ? idx - 1 : idx + 1;
  [sorted[idx], sorted[swapIdx]] = [sorted[swapIdx], sorted[idx]];
  try {
    await policyStore.reorder(sorted.map((r) => r.id));
    ElMessage.success("规则优先级已更新");
  } catch (error) {
    ElMessage.error(
      policyStore.lastError ||
        (error instanceof Error ? error.message : "更新规则优先级失败")
    );
  }
}

const editWeights = ref({ alpha: 0.35, beta: 0.40, gamma: 0.25 });
const editThreshold = ref(85);

onMounted(async () => {
  await policyStore.fetchConfig();
  if (policyStore.config) {
    editWeights.value = { ...policyStore.config.risk_weights };
    editThreshold.value = policyStore.config.global_threshold;
  }
});

async function saveWeights() {
  const total = editWeights.value.alpha + editWeights.value.beta + editWeights.value.gamma;
  if (Math.abs(total - 1.0) > 0.01) {
    ElMessage.warning("权重之和必须为 1.0");
    return;
  }
  try {
    await policyStore.saveConfig({
      risk_weights: { ...editWeights.value },
      global_threshold: editThreshold.value,
    });
    ElMessage.success("配置已保存");
  } catch (error) {
    ElMessage.error(
      policyStore.lastError ||
        (error instanceof Error ? error.message : "保存配置失败")
    );
  }
}

const conditionSyntaxTips = `
  支持标准正则表达式语法：
  · (?i) 表示不区分大小写
  · \\b 表示单词边界
  · .{0,80} 表示匹配 0-80 个任意字符
  · | 表示逻辑或
  示例: (?i)\\b(ignore|bypass|override)\\b.{0,80}\\b(system|instruction)\\b
`;

const ruleFormRules = {
  name: [{ required: true, message: "请填写规则名称", trigger: "blur" }],
  gate_type: [{ required: true, message: "请选择闸门类型", trigger: "change" }],
  action: [{ required: true, message: "请选择处置动作", trigger: "change" }],
};
</script>

<template>
  <div class="policy-management p-4">
    <h1 class="text-2xl font-bold mb-4">策略规则管理 / Policy Rule Management</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="4">
        <StatCard title="规则总数 / Total" :value="ruleStats.total" color="var(--aegis-primary)" />
      </el-col>
      <el-col :span="4">
        <StatCard title="启用规则 / Enabled" :value="ruleStats.enabled" color="var(--aegis-success)" />
      </el-col>
      <el-col :span="4">
        <StatCard title="拦截规则 / Blocking" :value="ruleStats.blocked" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="4">
        <StatCard title="消息闸门 / Message" :value="ruleStats.message" color="var(--aegis-info)" />
      </el-col>
      <el-col :span="4">
        <StatCard title="动作闸门 / Action" :value="ruleStats.action" color="var(--aegis-warning)" />
      </el-col>
      <el-col :span="4">
        <StatCard title="返回闸门 / Return" :value="ruleStats.returned" color="var(--aegis-primary)" />
      </el-col>
    </el-row>

    <el-card shadow="hover">
      <el-alert
        v-if="policyStore.lastError"
        :title="policyStore.lastError"
        type="warning"
        :closable="false"
        class="mb-4"
        description="当前页面未使用本地 mock 规则，请确认后端策略接口可用。"
      />
      <el-tabs v-model="activeTab">
        <el-tab-pane label="规则列表 / Rules" name="rules">
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm text-gray-500">
              共 {{ policyStore.rules.length }} 条规则，{{ ruleStats.enabled }} 条已启用
            </span>
            <el-button type="primary" @click="openCreateDialog">
              新增规则 / New Rule
            </el-button>
          </div>

          <el-table :data="policyStore.sortedRules" stripe size="small" max-height="700">
            <el-table-column type="index" label="#" width="50" />
            <el-table-column label="优先级 / Priority" width="80" align="center">
              <template #default="{ row }">
                <div class="flex items-center gap-1 justify-center">
                  <el-button
                    size="small"
                    link
                    :disabled="row.priority <= 1"
                    @click="movePriority(row, 'up')"
                  >
                    <el-icon><ArrowUp /></el-icon>
                  </el-button>
                  <span class="text-xs">{{ row.priority }}</span>
                  <el-button
                    size="small"
                    link
                    :disabled="row.priority >= policyStore.rules.length"
                    @click="movePriority(row, 'down')"
                  >
                    <el-icon><ArrowDown /></el-icon>
                  </el-button>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="名称 / Name" min-width="160">
              <template #default="{ row }">
                <div class="font-medium">{{ row.name }}</div>
                <div class="text-xs text-gray-400">{{ row.description }}</div>
              </template>
            </el-table-column>
            <el-table-column label="闸门 / Gate" width="120">
              <template #default="{ row }">
                <el-tag
                  size="small"
                  :type="row.gate_type === 'message' ? 'info' : row.gate_type === 'action' ? 'warning' : 'primary'"
                  effect="plain"
                >
                  {{ { message: "Message", action: "Action", return: "Return" }[row.gate_type] }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处置 / Action" width="110">
              <template #default="{ row }">
                <el-tag
                  size="small"
                  :type="row.action === 'Allow' ? 'success' : row.action === 'Block' || row.action === 'Deny' ? 'danger' : 'warning'"
                  effect="dark"
                >
                  {{ row.action }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="风险阈值 / Threshold" width="100" align="center">
              <template #default="{ row }">
                <span>{{ row.risk_threshold }}</span>
              </template>
            </el-table-column>
            <el-table-column label="状态 / Status" width="90" align="center">
              <template #default="{ row }">
                <el-switch
                  :model-value="row.enabled"
                  size="small"
                  :loading="policyStore.loading"
                  @click.stop
                  @change="toggleEnabled(row)"
                />
              </template>
            </el-table-column>
            <el-table-column label="操作 / Actions" width="150" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link @click="openEditDialog(row)">编辑</el-button>
                <el-button
                  size="small"
                  link
                  type="danger"
                  :disabled="!row.id.startsWith('rule-custom-')"
                  @click="handleDelete(row)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="!policyStore.rules.length && !policyStore.loading" class="text-center py-8">
            <el-empty description="暂无策略规则" :image-size="100" />
          </div>
        </el-tab-pane>

        <el-tab-pane label="风险权重配置 / Risk Weights" name="weights">
          <div class="max-w-2xl mx-auto py-4">
            <h3 class="text-lg font-semibold mb-4">风险评分权重 / Risk Score Weights</h3>
            <p class="text-sm text-gray-500 mb-6">
              风险评分 = α × 消息风险 + β × 动作风险 + γ × 返回风险。权重之和必须为 1.0。
            </p>

            <el-form label-position="top">
              <el-form-item label="消息风险权重 α (Message Gate Weight)">
                <el-slider
                  v-model="editWeights.alpha"
                  :min="0"
                  :max="1"
                  :step="0.01"
                  show-input
                  input-size="small"
                />
                <span class="text-xs text-gray-400 mt-1">当前值: {{ editWeights.alpha.toFixed(2) }}</span>
              </el-form-item>

              <el-form-item label="动作风险权重 β (Action Gate Weight)">
                <el-slider
                  v-model="editWeights.beta"
                  :min="0"
                  :max="1"
                  :step="0.01"
                  show-input
                  input-size="small"
                />
                <span class="text-xs text-gray-400 mt-1">当前值: {{ editWeights.beta.toFixed(2) }}</span>
              </el-form-item>

              <el-form-item label="返回风险权重 γ (Return Gate Weight)">
                <el-slider
                  v-model="editWeights.gamma"
                  :min="0"
                  :max="1"
                  :step="0.01"
                  show-input
                  input-size="small"
                />
                <span class="text-xs text-gray-400 mt-1">当前值: {{ editWeights.gamma.toFixed(2) }}</span>
              </el-form-item>

              <el-form-item label="权重总和">
                <el-tag
                  :type="Math.abs(editWeights.alpha + editWeights.beta + editWeights.gamma - 1.0) <= 0.01 ? 'success' : 'danger'"
                >
                  {{ (editWeights.alpha + editWeights.beta + editWeights.gamma).toFixed(2) }}
                </el-tag>
              </el-form-item>

              <el-divider />

              <el-form-item label="全局拦截阈值 (Global Block Threshold)">
                <el-slider
                  v-model="editThreshold"
                  :min="0"
                  :max="100"
                  :step="1"
                  show-input
                  input-size="small"
                />
                <span class="text-xs text-gray-400 mt-1">
                  风险评分超过此阈值时将触发拦截。当前值: {{ editThreshold }}
                </span>
              </el-form-item>

              <el-form-item>
                <el-button type="primary" :loading="policyStore.loading" @click="saveWeights">
                  保存配置 / Save Config
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>

        <el-tab-pane label="策略解释 / Explanation" name="explanation">
          <div class="py-4 max-w-3xl">
            <h3 class="text-lg font-semibold mb-4">策略引擎工作原理 / How Policy Engine Works</h3>

            <el-timeline>
              <el-timeline-item timestamp="1. 输入接收" placement="top" type="primary">
                <p class="text-sm">Agent 请求经过 Aegis 代理时，根据请求类型分发到对应的闸门通道。</p>
                <el-tag size="small" type="info" class="mt-2">Message Gate: 用户消息内容</el-tag>
                <el-tag size="small" type="warning" class="mt-2">Action Gate: 工具调用参数</el-tag>
                <el-tag size="small" type="primary" class="mt-2">Return Gate: 外部返回结果</el-tag>
              </el-timeline-item>

              <el-timeline-item timestamp="2. 策略匹配" placement="top" type="warning">
                <p class="text-sm">使用正则表达式逐条匹配已启用的策略规则。每条规则包含风险阈值和匹配条件。</p>
                <p class="text-xs text-gray-500 mt-1">规则按优先级排序，高优先级规则先匹配。</p>
              </el-timeline-item>

              <el-timeline-item timestamp="3. 风险评分" placement="top" type="danger">
                <p class="text-sm">匹配到的规则根据其风险阈值累加计算风险总分。不同闸门类型的分数按权重加权。</p>
                <p class="text-xs text-gray-500 mt-1">
                  风险总分 = α × 消息分 + β × 动作分 + γ × 返回分
                  其中 α + β + γ = 1.0
                </p>
              </el-timeline-item>

              <el-timeline-item timestamp="4. 处置决策" placement="top" type="success">
                <p class="text-sm">综合风险评分与全局阈值对比，输出最终处置决策。</p>
                <div class="mt-2 space-y-1">
                  <div class="flex items-center gap-2"><el-tag size="small" type="success">Allow</el-tag><span class="text-xs">评分低于阈值，正常放行</span></div>
                  <div class="flex items-center gap-2"><el-tag size="small" type="warning">Degrade</el-tag><span class="text-xs">评分接近阈值，降级处理（如限制工具调用范围）</span></div>
                  <div class="flex items-center gap-2"><el-tag size="small" type="danger">Block</el-tag><span class="text-xs">评分超过阈值，直接拦截</span></div>
                  <div class="flex items-center gap-2"><el-tag size="small" type="danger">Deny</el-tag><span class="text-xs">拒绝请求，返回错误</span></div>
                  <div class="flex items-center gap-2"><el-tag size="small" type="warning">HumanApproval</el-tag><span class="text-xs">高风险操作需要人工审批</span></div>
                </div>
              </el-timeline-item>
            </el-timeline>

            <el-alert
              title="提示"
              type="info"
              :closable="false"
              class="mt-4"
              description="策略配置修改后实时生效，无需重启服务。建议在修改高优先级规则前先在 Simulator 中测试效果。"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="isEditMode ? '编辑规则 / Edit Rule' : '新增规则 / New Rule'"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form
        :model="editingRule"
        :rules="ruleFormRules"
        label-position="top"
        size="small"
      >
        <el-row :gutter="16">
          <el-col :span="16">
            <el-form-item label="规则名称 / Rule Name" prop="name">
              <el-input v-model="editingRule.name" placeholder="请输入规则名称" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="优先级 / Priority">
              <el-input-number
                v-model="editingRule.priority"
                :min="1"
                :max="100"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="描述 / Description">
          <el-input
            v-model="editingRule.description"
            type="textarea"
            :rows="2"
            placeholder="规则描述（可选）"
          />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="闸门类型 / Gate Type" prop="gate_type">
              <el-select v-model="editingRule.gate_type" style="width: 100%">
                <el-option
                  v-for="opt in gateTypeOptions"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="处置动作 / Action" prop="action">
              <el-select v-model="editingRule.action" style="width: 100%">
                <el-option
                  v-for="opt in actionOptions"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="匹配条件 (正则表达式) / Condition">
          <el-input
            v-model="editingRule.condition"
            type="textarea"
            :rows="3"
            placeholder="(?i)\\b(pattern1|pattern2)\\b"
          />
          <div class="text-xs text-gray-400 mt-1">
            <el-popover placement="bottom" :width="400" trigger="click">
              <template #reference>
                <span class="cursor-pointer text-blue-500 hover:text-blue-700">语法提示</span>
              </template>
              <pre class="text-xs whitespace-pre-wrap">{{ conditionSyntaxTips }}</pre>
            </el-popover>
          </div>
        </el-form-item>

        <el-form-item label="风险阈值 / Risk Threshold">
          <el-slider
            v-model="editingRule.risk_threshold"
            :min="0"
            :max="100"
            :step="1"
            show-input
            input-size="small"
          />
          <span class="text-xs text-gray-400 mt-1">匹配此规则时累加的风险分数</span>
        </el-form-item>

        <el-form-item label="状态 / Status">
          <el-switch
            v-model="editingRule.enabled"
            active-text="启用 / Enabled"
            inactive-text="禁用 / Disabled"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消 / Cancel</el-button>
        <el-button type="primary" :loading="policyStore.loading" @click="saveRule">
          保存 / Save
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.policy-management :deep(.el-slider__input) {
  width: 80px;
}
</style>
