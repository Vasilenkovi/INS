package domain

// Severity — уровень критичности замечания.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityMajor    Severity = "major"
	SeverityMinor    Severity = "minor"
)

// Comment — одно замечание к строке кода.
type Comment struct {
	FilePath   string
	Line       int
	Severity   Severity
	Message    string
	Suggestion string
}

// FileDiff — diff одного файла в MR.
type FileDiff struct {
	OldPath  string
	NewPath  string
	Diff     string
	IsNew    bool
	IsDelete bool
}

// ReviewResult — итог анализа одного MR.
type ReviewResult struct {
	Comments     []Comment
	HasCritical  bool
	TotalByLevel map[Severity]int
	Verdict      string // "Пройдено" / "Требует исправления"
}
