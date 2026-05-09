package demo

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	RunID           string  `json:"run_id"`
	BenchmarkFamily string  `json:"benchmark_family"`
	BenchmarkSuite  string  `json:"benchmark_suite"`
	ASBAttack       string  `json:"asb_attack"`
	CaseID          string  `json:"case_id"`
	Scenario        string  `json:"scenario"`
	AgentName       string  `json:"agent_name"`
	Defense         string  `json:"defense"`
	AttackSuccess   bool    `json:"attack_success"`
	Refused         bool    `json:"refused"`
	TaskSuccess     bool    `json:"task_success"`
	LatencyMs       int     `json:"latency_ms"`
	ASR             float64 `json:"asr"`
	RR              float64 `json:"rr"`
	PNA             float64 `json:"pna"`
	BP              float64 `json:"bp"`
	FNR             float64 `json:"fnr"`
	FPR             float64 `json:"fpr"`
}

type ThreeGateResult struct {
	Case          string      `json:"case"`
	Title         string      `json:"title"`
	ExpectedFocus string      `json:"expected_focus"`
	UseLLM        bool        `json:"use_llm"`
	Stopped       bool        `json:"stopped"`
	FinalOutput   string      `json:"final_output"`
	Trace         []GateTrace `json:"trace"`
	GeneratedAt   string      `json:"generated_at"`
}

type GateTrace struct {
	Hard     GateVerdict     `json:"hard"`
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
	CombinedAction    string        `json:"combined_action"`
	CombinedRiskScore float64       `json:"combined_risk_score,omitempty"`
	DecisionBasis     string        `json:"decision_basis"`
	LLMJudgement      *LLMJudgement `json:"llm_judgement,omitempty"`
}

type LLMJudgement struct {
	RecommendedAction string  `json:"recommended_action"`
	RiskScore         float64 `json:"risk_score"`
	Reason            string  `json:"reason"`
}

type ExperimentManager struct {
	experimentsDir string
	langgraphDir   string
	aegisguardDir  string
}

func NewExperimentManager(rootDir string) *ExperimentManager {
	return &ExperimentManager{
		experimentsDir: filepath.Join(rootDir, "experiments", "asb", "results"),
		langgraphDir:   filepath.Join(rootDir, "ASB", "logs", "langgraph_batch"),
		aegisguardDir:  filepath.Join(rootDir, "experiments", "aegisguard", "results"),
	}
}

func (m *ExperimentManager) ListSummaries() ([]ExperimentSummary, error) {
	var summaries []ExperimentSummary

	adapterSummaries, err := m.listAdapterSummaries()
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, adapterSummaries...)

	langgraphSummaries, err := m.listLanggraphSummaries()
	if err != nil {
		return nil, err
	}
	summaries = append(summaries, langgraphSummaries...)

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Modified.After(summaries[j].Modified)
	})
	return summaries, nil
}

func (m *ExperimentManager) GetSummary(runID string) (map[string]interface{}, error) {
	if result, err := m.getJSONByRunID(m.experimentsDir, runID, "*-summary.json"); err == nil {
		return result, nil
	}

	path := filepath.Join(m.langgraphDir, runID+"-summary.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("summary not found for run_id: %s", runID)
	}

	var raw struct {
		RunID        string `json:"run_id"`
		LLMName      string `json:"llm_name"`
		AttackFamily string `json:"attack_family"`
		Aggregate    struct {
			Total                   int     `json:"total"`
			AttackSuccess           int     `json:"attack_success"`
			OriginalTaskSuccess     int     `json:"original_task_success"`
			RefusalCount            int     `json:"refusal_count"`
			AttackSuccessRate       float64 `json:"attack_success_rate"`
			OriginalTaskSuccessRate float64 `json:"original_task_success_rate"`
		} `json:"aggregate"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse JSON error: %w", err)
	}

	summary := map[string]interface{}{
		"run_id":    raw.RunID,
		"benchmark": "ASB",
		"attack":    raw.AttackFamily,
		"metrics": map[string]interface{}{
			"total":              raw.Aggregate.Total,
			"asr":                raw.Aggregate.AttackSuccessRate,
			"asr_count":          raw.Aggregate.AttackSuccess,
			"asr_total":          raw.Aggregate.Total,
			"rr":                 raw.Aggregate.OriginalTaskSuccessRate,
			"rr_count":           raw.Aggregate.OriginalTaskSuccess,
			"pna":                0,
			"bp":                 0,
			"fnr":                0,
			"fpr":                0,
			"average_latency_ms": 0,
		},
	}
	return summary, nil
}

func (m *ExperimentManager) GetRecords(runID string) ([]ExperimentRecord, error) {
	if records, err := m.getAdapterRecords(runID); err == nil {
		return records, nil
	}
	return m.getLanggraphRecords(runID)
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
	for _, item := range stats {
		runs := item["runs"].(int)
		item["avg_asr"] = item["asr_sum"].(float64) / float64(runs)
		item["avg_rr"] = item["rr_sum"].(float64) / float64(runs)
		delete(item, "asr_sum")
		delete(item, "rr_sum")
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i]["attack"].(string) < result[j]["attack"].(string)
	})
	return result, nil
}

func (m *ExperimentManager) listAdapterSummaries() ([]ExperimentSummary, error) {
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
			RunID     string         `json:"run_id"`
			Benchmark string         `json:"benchmark"`
			Attack    string         `json:"attack"`
			Metrics   SummaryMetrics `json:"metrics"`
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
			Metrics:   raw.Metrics,
		})
	}
	return summaries, nil
}

func (m *ExperimentManager) listLanggraphSummaries() ([]ExperimentSummary, error) {
	pattern := filepath.Join(m.langgraphDir, "*-summary.json")
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
			RunID        string `json:"run_id"`
			AttackFamily string `json:"attack_family"`
			Aggregate    struct {
				Total                   int     `json:"total"`
				AttackSuccess           int     `json:"attack_success"`
				OriginalTaskSuccess     int     `json:"original_task_success"`
				RefusalCount            int     `json:"refusal_count"`
				AttackSuccessRate       float64 `json:"attack_success_rate"`
				OriginalTaskSuccessRate float64 `json:"original_task_success_rate"`
			} `json:"aggregate"`
			Runs []struct {
				DurationSeconds float64 `json:"duration_seconds"`
			} `json:"runs"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		avgLatencyMs := 0.0
		totalDurationMs := 0.0
		for _, run := range raw.Runs {
			totalDurationMs += run.DurationSeconds * 1000
		}
		if raw.Aggregate.Total > 0 {
			avgLatencyMs = totalDurationMs / float64(raw.Aggregate.Total)
		}

		relPath, _ := filepath.Rel(filepath.Dir(m.langgraphDir), path)
		summaries = append(summaries, ExperimentSummary{
			RunID:     raw.RunID,
			Benchmark: "ASB",
			Attack:    raw.AttackFamily,
			File:      relPath,
			Modified:  info.ModTime(),
			Metrics: SummaryMetrics{
				Total:            raw.Aggregate.Total,
				ASR:              raw.Aggregate.AttackSuccessRate,
				ASRCount:         raw.Aggregate.AttackSuccess,
				ASRTotal:         raw.Aggregate.Total,
				RR:               raw.Aggregate.OriginalTaskSuccessRate,
				RRCount:          raw.Aggregate.OriginalTaskSuccess,
				PNA:              0,
				BP:               0,
				FNR:              0,
				FPR:              0,
				AverageLatencyMs: avgLatencyMs,
			},
		})
	}
	return summaries, nil
}

func (m *ExperimentManager) getJSONByRunID(dir, runID, suffix string) (map[string]interface{}, error) {
	pattern := filepath.Join(dir, "*"+runID+"*"+suffix)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}
	if len(matches) == 0 {
		pattern = filepath.Join(dir, runID+"*"+suffix)
		matches, err = filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob error: %w", err)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("not found")
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

func (m *ExperimentManager) getAdapterRecords(runID string) ([]ExperimentRecord, error) {
	pattern := filepath.Join(m.experimentsDir, "*"+runID+"*-results.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}
	if len(matches) == 0 {
		pattern = filepath.Join(m.experimentsDir, runID+"*-results.csv")
		matches, err = filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob error: %w", err)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("results CSV not found for run_id: %s", runID)
	}
	return parseAdapterRecords(matches[0])
}

func parseAdapterRecords(path string) ([]ExperimentRecord, error) {
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

		records = append(records, ExperimentRecord{
			RunID:           get("run_id"),
			BenchmarkFamily: get("benchmark_family"),
			BenchmarkSuite:  get("benchmark_suite"),
			ASBAttack:       get("asb_attack"),
			CaseID:          get("case_id"),
			Scenario:        get("scenario"),
			AgentName:       get("agent_name"),
			Defense:         get("defense"),
			AttackSuccess:   parseBool(get("attack_success")),
			Refused:         parseBool(get("refused")),
			TaskSuccess:     parseBool(get("task_success")),
			LatencyMs:       parseInt(get("latency_ms")),
			ASR:             parseFloat(get("asr")),
			RR:              parseFloat(get("rr")),
			PNA:             parseFloat(get("pna")),
			BP:              parseFloat(get("bp")),
			FNR:             parseFloat(get("fnr")),
			FPR:             parseFloat(get("fpr")),
		})
	}

	return records, nil
}

func (m *ExperimentManager) getLanggraphRecords(runID string) ([]ExperimentRecord, error) {
	path := filepath.Join(m.langgraphDir, runID+"-cases.csv")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("results CSV not found for run_id: %s", runID)
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

	get := func(row []string, name string) string {
		if idx, ok := headerIndex[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	var records []ExperimentRecord
	index := 0
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		index++
		attackType := get(row, "attack_type")
		attackSuccess := parseBool(get(row, "attack_successful"))
		taskSuccess := parseBool(get(row, "original_task_successful"))
		refused := parseBool(get(row, "refuse_result"))

		records = append(records, ExperimentRecord{
			RunID:           get(row, "run_id"),
			BenchmarkFamily: "ASB",
			BenchmarkSuite:  get(row, "attack_family"),
			ASBAttack:       get(row, "attack_family"),
			CaseID:          fmt.Sprintf("%s-%03d", runID, index),
			Scenario:        attackType + " / " + get(row, "attack_tool"),
			AgentName:       get(row, "agent_name"),
			Defense:         inferDefenseFromRunID(runID),
			AttackSuccess:   attackSuccess,
			Refused:         refused,
			TaskSuccess:     taskSuccess,
			LatencyMs:       0,
			ASR:             boolToFloat(attackSuccess),
			RR:              boolToFloat(taskSuccess),
			PNA:             0,
			BP:              0,
			FNR:             0,
			FPR:             0,
		})
	}

	return records, nil
}

func inferDefenseFromRunID(runID string) string {
	lower := strings.ToLower(runID)
	if strings.Contains(lower, "gated") || strings.Contains(lower, "aegisguard") {
		if strings.Contains(lower, "nodefense") {
			return "none"
		}
		return "aegisguard_gate"
	}
	return "none"
}

func parseBool(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return v == "true" || v == "1" || v == "yes"
}

func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseInt(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
