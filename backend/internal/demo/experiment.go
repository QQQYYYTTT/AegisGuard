package demo

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ExperimentSummary struct {
	RunID     string         `json:"run_id"`
	Benchmark string         `json:"benchmark"`
	Attack    string         `json:"attack"`
	Metrics   SummaryMetrics `json:"metrics"`
	File      string         `json:"file"`
	Modified  time.Time      `json:"modified"`
}

type SummaryMetrics struct {
	Total            int     `json:"total"`
	ASR              float64 `json:"asr"`
	ASRCount         int     `json:"asr_count"`
	ASRTotal         int     `json:"asr_total"`
	RR               float64 `json:"rr"`
	RRCount          int     `json:"rr_count"`
	PNA              float64 `json:"pna"`
	BP               float64 `json:"bp"`
	FNR              float64 `json:"fnr"`
	FPR              float64 `json:"fpr"`
	AverageLatencyMs float64 `json:"average_latency_ms"`
}

type ExperimentRecord struct {
	RunID                string  `json:"run_id"`
	BenchmarkFamily      string  `json:"benchmark_family"`
	BenchmarkSuite       string  `json:"benchmark_suite"`
	ASBAttack            string  `json:"asb_attack"`
	CaseID               string  `json:"case_id"`
	Scenario             string  `json:"scenario"`
	AgentName            string  `json:"agent_name"`
	Defense              string  `json:"defense"`
	AttackSuccess        bool    `json:"attack_success"`
	Refused              bool    `json:"refused"`
	TaskSuccess          bool    `json:"task_success"`
	LatencyMs            int     `json:"latency_ms"`
	ASR                  float64 `json:"asr"`
	RR                   float64 `json:"rr"`
	PNA                  float64 `json:"pna"`
	BP                   float64 `json:"bp"`
	FNR                  float64 `json:"fnr"`
	FPR                  float64 `json:"fpr"`
}

type ThreeGateResult struct {
	Case          string        `json:"case"`
	Title         string        `json:"title"`
	ExpectedFocus string        `json:"expected_focus"`
	UseLLM        bool          `json:"use_llm"`
	Stopped       bool          `json:"stopped"`
	FinalOutput   string        `json:"final_output"`
	Trace         []GateTrace   `json:"trace"`
	GeneratedAt   string        `json:"generated_at"`
}

type GateTrace struct {
	Hard     GateVerdict `json:"hard"`
	Combined CombinedVerdict `json:"combined"`
}

type GateVerdict struct {
	Stage          string   `json:"stage"`
	Action         string   `json:"action"`
	Reason         string   `json:"reason"`
	TriggeredRules []string `json:"triggered_rules"`
	RiskScore      float64  `json:"risk_score"`
	DecisionBasis  string   `json:"decision_basis"`
	SafeText       string   `json:"safe_text,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
}

type CombinedVerdict struct {
	CombinedAction   string        `json:"combined_action"`
	CombinedRiskScore float64      `json:"combined_risk_score,omitempty"`
	DecisionBasis    string        `json:"decision_basis"`
	LLMJudgement     *LLMJudgement `json:"llm_judgement,omitempty"`
}

type LLMJudgement struct {
	RecommendedAction string  `json:"recommended_action"`
	RiskScore         float64 `json:"risk_score"`
	Reason            string  `json:"reason"`
}

type ExperimentManager struct {
	experimentsDir string
	aegisguardDir  string
}

func NewExperimentManager(rootDir string) *ExperimentManager {
	return &ExperimentManager{
		experimentsDir: filepath.Join(rootDir, "experiments", "asb", "results"),
		aegisguardDir:  filepath.Join(rootDir, "experiments", "aegisguard", "results"),
	}
}

func (m *ExperimentManager) ListSummaries() ([]ExperimentSummary, error) {
	pattern := filepath.Join(m.experimentsDir, "*-summary.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	var summaries []ExperimentSummary
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var raw struct {
			RunID     string `json:"run_id"`
			Benchmark string `json:"benchmark"`
			Attack    string `json:"attack"`
			Metrics   struct {
				Total            int     `json:"total"`
				ASR              float64 `json:"asr"`
				ASRCount         int     `json:"asr_count"`
				ASRTotal         int     `json:"asr_total"`
				RR               float64 `json:"rr"`
				RRCount          int     `json:"rr_count"`
				PNA              float64 `json:"pna"`
				BP               float64 `json:"bp"`
				FNR              float64 `json:"fnr"`
				FPR              float64 `json:"fpr"`
				AverageLatencyMs float64 `json:"average_latency_ms"`
			} `json:"metrics"`
		}

		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		relPath, _ := filepath.Rel(filepath.Dir(m.experimentsDir), path)
		summaries = append(summaries, ExperimentSummary{
			RunID:     raw.RunID,
			Benchmark: raw.Benchmark,
			Attack:    raw.Attack,
			File:      relPath,
			Modified:  info.ModTime(),
			Metrics: SummaryMetrics{
				Total:            raw.Metrics.Total,
				ASR:              raw.Metrics.ASR,
				ASRCount:         raw.Metrics.ASRCount,
				ASRTotal:         raw.Metrics.ASRTotal,
				RR:               raw.Metrics.RR,
				RRCount:          raw.Metrics.RRCount,
				PNA:              raw.Metrics.PNA,
				BP:               raw.Metrics.BP,
				FNR:              raw.Metrics.FNR,
				FPR:              raw.Metrics.FPR,
				AverageLatencyMs: raw.Metrics.AverageLatencyMs,
			},
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Modified.After(summaries[j].Modified)
	})

	return summaries, nil
}

func (m *ExperimentManager) GetSummary(runID string) (map[string]interface{}, error) {
	pattern := filepath.Join(m.experimentsDir, "*"+runID+"*-summary.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	if len(matches) == 0 {
		pattern2 := filepath.Join(m.experimentsDir, runID+"*-summary.json")
		matches, err = filepath.Glob(pattern2)
		if err != nil {
			return nil, fmt.Errorf("glob error: %w", err)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("summary not found for run_id: %s", runID)
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse JSON error: %w", err)
	}

	return result, nil
}

func (m *ExperimentManager) GetRecords(runID string) ([]ExperimentRecord, error) {
	pattern := filepath.Join(m.experimentsDir, "*"+runID+"*-results.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	if len(matches) == 0 {
		pattern2 := filepath.Join(m.experimentsDir, runID+"*-results.csv")
		matches, err = filepath.Glob(pattern2)
		if err != nil {
			return nil, fmt.Errorf("glob error: %w", err)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("results CSV not found for run_id: %s", runID)
	}

	return parseCSVRecords(matches[0])
}

func parseCSVRecords(path string) ([]ExperimentRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV error: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read headers error: %w", err)
	}

	headerIndex := make(map[string]int)
	for i, h := range headers {
		headerIndex[strings.TrimSpace(h)] = i
	}

	var records []ExperimentRecord
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		get := func(name string) string {
			if idx, ok := headerIndex[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		getBool := func(name string) bool {
			v := strings.ToLower(get(name))
			return v == "true" || v == "1" || v == "yes"
		}

		getFloat := func(name string) float64 {
			var f float64
			v := get(name)
			if v != "" {
				fmt.Sscanf(v, "%f", &f)
			}
			return f
		}

		getInt := func(name string) int {
			var i int
			v := get(name)
			if v != "" {
				fmt.Sscanf(v, "%d", &i)
			}
			return i
		}

		records = append(records, ExperimentRecord{
			RunID:           get("run_id"),
			BenchmarkFamily: get("benchmark_family"),
			BenchmarkSuite:  get("benchmark_suite"),
			ASBAttack:       get("asb_attack"),
			CaseID:          get("case_id"),
			Scenario:        get("scenario"),
			AgentName:       get("agent_name"),
			Defense:         get("defense"),
			AttackSuccess:   getBool("attack_success"),
			Refused:         getBool("refused"),
			TaskSuccess:     getBool("task_success"),
			LatencyMs:       getInt("latency_ms"),
			ASR:             getFloat("asr"),
			RR:              getFloat("rr"),
			PNA:             getFloat("pna"),
			BP:              getFloat("bp"),
			FNR:             getFloat("fnr"),
			FPR:             getFloat("fpr"),
		})
	}

	return records, nil
}

func (m *ExperimentManager) GetThreeGateResult() (*ThreeGateResult, error) {
	path := filepath.Join(m.aegisguardDir, "three_gate_demo_last.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}

	var result ThreeGateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse JSON error: %w", err)
	}

	return &result, nil
}

func (m *ExperimentManager) GetAttackFamilyStats() ([]map[string]interface{}, error) {
	summaries, err := m.ListSummaries()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]map[string]interface{})
	for _, s := range summaries {
		key := s.Attack
		if _, ok := stats[key]; !ok {
			stats[key] = map[string]interface{}{
				"attack":      s.Attack,
				"runs":        0,
				"total_cases": 0,
				"avg_asr":     0.0,
				"avg_rr":      0.0,
				"asr_sum":     0.0,
				"rr_sum":      0.0,
			}
		}
		stats[key]["runs"] = stats[key]["runs"].(int) + 1
		stats[key]["total_cases"] = stats[key]["total_cases"].(int) + s.Metrics.Total
		stats[key]["asr_sum"] = stats[key]["asr_sum"].(float64) + s.Metrics.ASR
		stats[key]["rr_sum"] = stats[key]["rr_sum"].(float64) + s.Metrics.RR
	}

	var result []map[string]interface{}
	for _, v := range stats {
		runs := v["runs"].(int)
		v["avg_asr"] = v["asr_sum"].(float64) / float64(runs)
		v["avg_rr"] = v["rr_sum"].(float64) / float64(runs)
		delete(v, "asr_sum")
		delete(v, "rr_sum")
		result = append(result, v)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i]["attack"].(string) < result[j]["attack"].(string)
	})

	return result, nil
}
