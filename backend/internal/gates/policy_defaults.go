package gates

func defaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		RiskWeights: RiskWeightsConfig{
			Alpha: 0.35,
			Beta:  0.40,
			Gamma: 0.25,
		},
		GlobalThreshold: 85,
		Rules: []PolicyRuleConfig{
			{ID: "prompt_injection", Name: "Prompt Injection 检测", Description: "检测提示注入攻击，包含忽略/绕过系统指令等模式", GateType: "message", Condition: `(?i)\b(ignore|forget|bypass|override)\b.{0,80}\b(previous|prior|system|developer|instruction|policy|rule)s?\b|\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection)\b`, Action: "Block", Priority: 1, Enabled: true, RiskThreshold: 35},
			{ID: "privileged_scope", Name: "越权操作检测", Description: "检测企图获取管理员权限、执行敏感系统命令的行为", GateType: "action", Condition: `(?i)\b(admin|root|sudo|privileged|permission|credential|authorization|access token)\b|\b(shell|cmd|powershell|bash|exec|spawn|subprocess)\b|\b(delete|drop|wipe|exfiltrate|upload|download|modify|overwrite)\b.{0,80}\b(file|database|record|config|secret)\b`, Action: "Block", Priority: 2, Enabled: true, RiskThreshold: 30},
			{ID: "sensitive_access", Name: "敏感数据泄露防护", Description: "保护 API Key、密码、Token 等敏感凭证不被泄露", GateType: "return", Condition: `(?i)\b(api[_-]?key|api\s+key|password|passwd|secret|private key|token|session|cookie|credential)\b|\b(sk-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|Bearer\s+[A-Za-z0-9._-]{12,})\b|\b(card number|credit card|ssn|social security|id card|bank account)\b`, Action: "Block", Priority: 3, Enabled: true, RiskThreshold: 35},
			{ID: "high_impact_action", Name: "高危操作拦截", Description: "拦截生产环境删除、转账、支付等高危操作", GateType: "action", Condition: `(?i)\b(transfer|wire|withdraw|purchase|refund|pay|trade|sell|buy|delete|disable|revoke)\b.{0,80}\b(production|prod|customer|account|database|billing|payment|funds?|money|record|file)\b|\b(transfer_funds|wire_transfer|delete_file|shell_exec)\b|\b(rm\s+-rf|format\s+[A-Z]:|drop\s+table|truncate\s+table)\b`, Action: "HumanApproval", Priority: 4, Enabled: true, RiskThreshold: 32},
			{ID: "memory_poisoning", Name: "记忆投毒防护", Description: "检测并阻止试图污染 Agent 长期记忆的恶意指令", GateType: "return", Condition: `(?i)\b(save|store|remember|persist|write)\b.{0,80}\b(instruction|rule|memory|policy|system prompt)\b|\b(remember|save|store|persist)\b.{0,80}\b(command|response|answer)\b.{0,80}\b(forever|always|from now on)\b|\b(add this to memory|update your memory|from now on)\b`, Action: "Block", Priority: 5, Enabled: true, RiskThreshold: 45},
			{ID: "illegal_finance", Name: "非法金融活动检测", Description: "检测洗钱、内幕交易、逃税等非法金融活动模式", GateType: "message", Condition: `(?i)\b(money laundering|insider trading|fake invoice|tax evasion|evade sanctions)\b|\b(fraudulent|stolen card|bypass kyc|launder)\b`, Action: "Deny", Priority: 6, Enabled: true, RiskThreshold: 70},
			{ID: "rule-7", Name: "正常请求放行", Description: "允许所有不匹配任何高危模式的正常用户请求", GateType: "message", Condition: "", Action: "Allow", Priority: 7, Enabled: true, RiskThreshold: 0},
			{ID: "rule-8", Name: "回调污染检测", Description: "检测外部工具返回结果中注入恶意指令的回调污染攻击", GateType: "return", Condition: `(?i)\b(observation injection|tool output|external content)\b.{0,80}\b(instruction|command|override)\b`, Action: "Degrade", Priority: 8, Enabled: true, RiskThreshold: 25},
			{ID: "rule-9", Name: "重放攻击检测", Description: "识别会话重放、重复提权操作等重放攻击行为", GateType: "action", Condition: `(?i)\b(replay|repeat|again|retry)\b.{0,80}\b(privileged|action|export|admin)\b`, Action: "Deny", Priority: 9, Enabled: true, RiskThreshold: 40},
			{ID: "rule-10", Name: "工具误用检测", Description: "检测 Agent 工具调用超出合理范围的行为", GateType: "action", Condition: `(?i)\b(delete_file|shell_exec|drop_table|rm\s+-rf|format)\b`, Action: "Block", Priority: 10, Enabled: true, RiskThreshold: 50},
			{ID: "prompt_injection_return", Name: "返回注入污染检测", Description: "检测回传结果中的提示词注入与策略覆盖内容", GateType: "return", Condition: `(?i)\b(ignore|forget|bypass|override)\b.{0,80}\b(previous|prior|system|developer|instruction|policy|rule)s?\b|\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection)\b`, Action: "Degrade", Priority: 11, Enabled: true, RiskThreshold: 35},
			{ID: "sensitive_access_message", Name: "输入敏感信息访问检测", Description: "检测输入中对凭证、密钥、敏感身份数据的索取", GateType: "message", Condition: `(?i)\b(api[_-]?key|api\s+key|password|passwd|secret|private key|token|session|cookie|credential)\b|\b(sk-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|Bearer\s+[A-Za-z0-9._-]{12,})\b|\b(card number|credit card|ssn|social security|id card|bank account)\b`, Action: "Degrade", Priority: 12, Enabled: true, RiskThreshold: 35},
			{ID: "memory_poisoning_message", Name: "输入记忆投毒检测", Description: "检测输入中试图持久化恶意指令到系统记忆的内容", GateType: "message", Condition: `(?i)\b(save|store|remember|persist|write)\b.{0,80}\b(instruction|rule|memory|policy|system prompt)\b|\b(remember|save|store|persist)\b.{0,80}\b(command|response|answer)\b.{0,80}\b(forever|always|from now on)\b|\b(add this to memory|update your memory|from now on)\b`, Action: "Block", Priority: 13, Enabled: true, RiskThreshold: 45},
			{ID: "illegal_finance_return", Name: "返回非法金融内容检测", Description: "检测工具或模型返回中的非法金融活动指导内容", GateType: "return", Condition: `(?i)\b(money laundering|insider trading|fake invoice|tax evasion|evade sanctions)\b|\b(fraudulent|stolen card|bypass kyc|launder)\b`, Action: "Deny", Priority: 14, Enabled: true, RiskThreshold: 70},
		},
	}
}
