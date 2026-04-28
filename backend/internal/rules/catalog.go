package catalog

type ExperimentLayer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Goal        string `json:"goal"`
}

type Agent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	Family         string   `json:"family"`
	Abilities      []string `json:"abilities"`
	NativeSecurity []string `json:"nativeSecurity"`
	TestFocus      []string `json:"testFocus"`
	Status         string   `json:"status"`
}

type AttackFamily struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Target   string   `json:"target"`
	Variants []string `json:"variants"`
	Gate     string   `json:"gate"`
}

type ScenarioTemplate struct {
	UserGoal       string `json:"userGoal"`
	AgentID        string `json:"agentId"`
	SessionID      string `json:"sessionId"`
	ToolName       string `json:"toolName"`
	Scope          string `json:"scope"`
	RequestedScope string `json:"requestedScope"`
	RawResult      string `json:"rawResult"`
}

type ExperimentPlan struct {
	Stage         string         `json:"stage"`
	Focus         string         `json:"focus"`
	Dimensions    map[string]any `json:"dimensions"`
	SuggestedRuns map[string]int `json:"suggestedRuns"`
}

type ExperimentAssets struct {
	Overview    string   `json:"overview"`
	Directories []string `json:"directories"`
	RecordTypes []string `json:"recordTypes"`
}

var Layers = []ExperimentLayer{
	{
		ID:          "asb",
		Name:        "ASB benchmark",
		Description: "Run the original Agent Security Benchmark from an external checkout and convert its outputs into AegisGuard records.",
		Goal:        "Evaluate AegisGuard against ASB tasks, attacks, tools, and output records without local benchmark code.",
	},
	{
		ID:          "aegisguard",
		Name:        "ASB + AegisGuard",
		Description: "Run the same ASB benchmark with AegisGuard runtime controls enabled.",
		Goal:        "Measure attack success, utility, latency, and traceability under AegisGuard protection.",
	},
}

var Agents = []Agent{
	{
		ID:             "asb-agent",
		Name:           "ASB configured agents",
		Category:       "Benchmark agents",
		Family:         "asb",
		Abilities:      []string{"ASB tasks", "ASB tools", "ASB attack and defense configs"},
		NativeSecurity: []string{"Defined by the selected ASB configuration"},
		TestFocus:      []string{"Prompt injection", "Observation injection", "Memory poisoning", "Planning backdoor", "Mixed attacks", "Tool misuse"},
		Status:         "active",
	},
}

var AttackFamilies = []AttackFamily{
	{
		ID:       "dpi",
		Name:     "Direct Prompt Injection",
		Target:   "User query and instruction hierarchy",
		Variants: []string{"ASB DPI config"},
		Gate:     "Message Gate",
	},
	{
		ID:       "opi",
		Name:     "Observation Prompt Injection",
		Target:   "External observations, tool outputs, pages, files, and environment text",
		Variants: []string{"ASB OPI config"},
		Gate:     "Return Gate",
	},
	{
		ID:       "mp",
		Name:     "Memory Poisoning",
		Target:   "Agent memory retrieval and persistent context",
		Variants: []string{"ASB MP config"},
		Gate:     "Return Gate",
	},
	{
		ID:       "pot",
		Name:     "Plan-of-Thought Backdoor",
		Target:   "Planning and hidden trigger behavior",
		Variants: []string{"ASB PoT config"},
		Gate:     "Message Gate",
	},
	{
		ID:       "mixed",
		Name:     "Mixed Attack",
		Target:   "Multi-stage agent compromise across prompts, observations, tools, and memory",
		Variants: []string{"ASB mixed config"},
		Gate:     "Message/Return/Action Gate",
	},
}

var ScenarioTemplates = map[string]ScenarioTemplate{
	"asb-opi": {
		UserGoal:       "Run ASB observation prompt injection through the external ASB checkout.",
		AgentID:        "asb-agent",
		SessionID:      "asb-opi-v1",
		ToolName:       "experiments/asb/run_asb.py",
		Scope:          "benchmark.run",
		RequestedScope: "benchmark.run",
		RawResult:      "python .\\experiments\\asb\\run_asb.py --asb-root F:\\2026信安赛\\ASB --attack opi --run-id asb-opi-v1",
	},
	"asb-convert": {
		UserGoal:       "Convert ASB output files into the AegisGuard evaluation schema.",
		AgentID:        "asb-agent",
		SessionID:      "asb-opi-v1",
		ToolName:       "experiments/asb/collect_results.py",
		Scope:          "benchmark.convert",
		RequestedScope: "benchmark.convert",
		RawResult:      "python .\\experiments\\asb\\collect_results.py --input F:\\2026信安赛\\ASB\\logs --attack opi --run-id asb-opi-v1",
	},
}

var Assets = ExperimentAssets{
	Overview:    "AegisGuard experiments are ASB-first. Local pilot benchmark code and results have been removed.",
	Directories: []string{"experiments/asb", "experiments/eval", "experiments/aegisguard"},
	RecordTypes: []string{"ASB manifests", "converted CSV results", "summary JSON", "trace JSON"},
}

func BuildPlan() ExperimentPlan {
	return ExperimentPlan{
		Stage: "ASB benchmark integration",
		Focus: "Run original ASB attacks and convert results into the AegisGuard schema.",
		Dimensions: map[string]any{
			"benchmark":      "ASB",
			"attackFamilies": len(AttackFamilies),
			"layers":         len(Layers),
		},
		SuggestedRuns: map[string]int{
			"asbAttackConfigs": len(AttackFamilies),
			"minimumRepeats":   3,
		},
	}
}
