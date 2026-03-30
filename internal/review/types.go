package review

import "github.com/DreamCats/coco-ext/internal/git"

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
