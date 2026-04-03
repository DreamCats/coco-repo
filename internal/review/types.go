package review

import (
	"github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/lint"
)

type Severity string

const (
	SeverityP0 Severity = "P0"
	SeverityP1 Severity = "P1"
	SeverityP2 Severity = "P2"
)

type Finding struct {
	Severity   Severity
	Source     string
	File       string
	Title      string
	Detail     string
	Suggestion string
}

type FileChange struct {
	Path              string
	Additions         int
	Deletions         int
	Category          string
	TopDir            string
	HasTestSibling    bool
	PublicSigAdded    []string
	PublicSigRemoved  []string
	AddedTodoLines    []string
	AddedSensitive    []string
	AddedDDL          []string
	IsConfigLike      bool
	IsIDLLike         bool
	IsDocLike         bool
	IsGoLike          bool
	IsTestLike        bool
}

type Facts struct {
	DiffInfo              *git.DiffInfo
	Files                 []FileChange
	GoFileCount           int
	TODOCount             int
	FIXMECount            int
	HACKCount             int
	SensitiveFindings     []string
	ConfigFiles           []string
	IDLFiles              []string
	DDLFindings           []string
	PublicSignatureChange []string
	MissingTests          []string
	LargeFiles            []string
	TotalChangedLines     int
	LintIssues            []lint.LintIssue
	LintOutputDir         string
}

type ScopeResult struct {
	Findings []Finding
	Core     []string
	Edge     []string
	Outliers []string
}

type ReleaseResult struct {
	Findings []Finding
}

type ImpactResult struct {
	Findings []Finding
	Skipped  bool
	Reason   string
}

type QualityResult struct {
	Summary  string
	Findings []Finding
	Raw      string
}

type ReportInputs struct {
	Facts   Facts
	Scope   ScopeResult
	Release ReleaseResult
	Impact  ImpactResult
	Quality QualityResult
}

type ReviewSummary struct {
	Rating       string    `json:"rating"`
	Advice       string    `json:"advice"`
	P0Count      int       `json:"p0_count"`
	P1Count      int       `json:"p1_count"`
	P2Count      int       `json:"p2_count"`
	TotalFindings int      `json:"total_findings"`
	GeneratedAt  string    `json:"generated_at"`
}

type PipelineResult struct {
	Facts    Facts         `json:"facts"`
	Scope    ScopeResult   `json:"scope"`
	Release  ReleaseResult `json:"release"`
	Impact   ImpactResult  `json:"impact"`
	Quality  QualityResult `json:"quality"`
	Summary  ReviewSummary `json:"summary"`
	ReportMD string        `json:"report_md"`
}
