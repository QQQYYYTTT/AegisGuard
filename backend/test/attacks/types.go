package attacks

import "aegisguard/backend/internal/rules"

type SourceRef struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Link  string `json:"link"`
	Notes string `json:"notes"`
}

type PayloadTemplate struct {
	UserGoal          string `json:"userGoal"`
	AttackPrompt      string `json:"attackPrompt"`
	ToolName          string `json:"toolName"`
	Scope             string `json:"scope"`
	RequestedScope    string `json:"requestedScope"`
	RawResult         string `json:"rawResult"`
	SeedMemory        string `json:"seedMemory"`
	ToolMetadata      string `json:"toolMetadata"`
	AttachmentName    string `json:"attachmentName"`
	AttachmentExcerpt string `json:"attachmentExcerpt"`
}

type AttackCase struct {
	ID              string          `json:"id"`
	FamilyID        string          `json:"familyId"`
	VariantID       string          `json:"variantId"`
	VariantName     string          `json:"variantName"`
	Title           string          `json:"title"`
	Severity        string          `json:"severity"`
	Objective       string          `json:"objective"`
	Scenario        string          `json:"scenario"`
	TargetStage     string          `json:"targetStage"`
	AppliesTo       []string        `json:"appliesTo"`
	Setup           []string        `json:"setup"`
	ExecutionSteps  []string        `json:"executionSteps"`
	SuccessCriteria []string        `json:"successCriteria"`
	SafeOutcome     []string        `json:"safeOutcome"`
	ExpectedSignals []string        `json:"expectedSignals"`
	ManualChecks    []string        `json:"manualChecks"`
	InspiredBy      []SourceRef     `json:"inspiredBy"`
	Payload         PayloadTemplate `json:"payload"`
	Tags            []string        `json:"tags"`
}

type FamilyBundle struct {
	FamilyID   string       `json:"familyId"`
	FamilyName string       `json:"familyName"`
	Cases      []AttackCase `json:"cases"`
}

type Library struct {
	Overview      string                 `json:"overview"`
	Families      []rules.AttackFamily `json:"families"`
	Cases         []AttackCase           `json:"cases"`
	CasesByFamily []FamilyBundle         `json:"casesByFamily"`
}
