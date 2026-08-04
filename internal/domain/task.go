package domain

import (
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	TaskProfileSchemaVersion = "task-profile/v1"
	TaskRuleVersion          = "l1-v1"
	TaskFamilyVersion        = "v1"
	DifficultyVersion        = "v1"
)

type TaskFamily string

const (
	TaskFamilyGeneralQA                 TaskFamily = "general_qa"
	TaskFamilyClassification            TaskFamily = "classification"
	TaskFamilyExtraction                TaskFamily = "extraction"
	TaskFamilyDocumentation             TaskFamily = "documentation"
	TaskFamilySummarization             TaskFamily = "summarization"
	TaskFamilyCodeGeneration            TaskFamily = "code_generation"
	TaskFamilyCodeReview                TaskFamily = "code_review"
	TaskFamilyDebugging                 TaskFamily = "debugging"
	TaskFamilyArchitectureDesign        TaskFamily = "architecture_design"
	TaskFamilyTestGeneration            TaskFamily = "test_generation"
	TaskFamilyErrorRecovery             TaskFamily = "error_recovery"
	TaskFamilyDataAnalysis              TaskFamily = "data_analysis"
	TaskFamilyToolUsePlanning           TaskFamily = "tool_use_planning"
	TaskFamilyKubernetesTroubleshooting TaskFamily = "kubernetes_troubleshooting"
)

type Difficulty int

const (
	DifficultyUnknown Difficulty = iota
	DifficultyOne
	DifficultyTwo
	DifficultyThree
	DifficultyFour
	DifficultyFive
)

type Domain string

const (
	DomainUnknown             Domain = "unknown"
	DomainKubernetes          Domain = "kubernetes"
	DomainSoftwareEngineering Domain = "software_engineering"
	DomainData                Domain = "data"
)

type RiskLevel string

const (
	RiskUnknown RiskLevel = "unknown"
	RiskHigh    RiskLevel = "high"
)

type Seriousness string

const (
	SeriousnessUnknown Seriousness = "unknown"
	SeriousnessHigh    Seriousness = "high"
)

type ContextSize string

const (
	ContextTiny      ContextSize = "tiny"
	ContextSmall     ContextSize = "small"
	ContextMedium    ContextSize = "medium"
	ContextLarge     ContextSize = "large"
	ContextVeryLarge ContextSize = "very_large"
)

type Capability string

const (
	CapabilityTools            Capability = "tools"
	CapabilityStructuredOutput Capability = "structured_output"
	CapabilityFileContext      Capability = "file_context"
)

type VerificationMode string

const (
	VerificationUnknown VerificationMode = "unknown"
	VerificationSchema  VerificationMode = "schema"
	VerificationTests   VerificationMode = "tests"
)

// Classification records the value and local facts that produced it.
type Classification[T any] struct {
	Value      T        `json:"value"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

// TaskProfile is an immutable, versioned result of the local task classifier.
type TaskProfile struct {
	SchemaVersion        string                           `json:"schema_version"`
	RuleVersion          string                           `json:"rule_version"`
	TaskFamilyVersion    string                           `json:"task_family_version"`
	DifficultyVersion    string                           `json:"difficulty_version"`
	TaskFamily           Classification[TaskFamily]       `json:"task_family"`
	Difficulty           Classification[Difficulty]       `json:"difficulty"`
	Domain               Classification[Domain]           `json:"domain"`
	RiskLevel            Classification[RiskLevel]        `json:"risk_level"`
	Seriousness          Classification[Seriousness]      `json:"seriousness"`
	ContextSize          Classification[ContextSize]      `json:"context_size"`
	RequiredCapabilities Classification[[]Capability]     `json:"required_capabilities"`
	VerificationMode     Classification[VerificationMode] `json:"verification_mode"`
	TokenEstimate        Classification[int]              `json:"token_estimate"`
	ToolDeclarations     Classification[[]string]         `json:"tool_declarations"`
	FileTypes            Classification[[]string]         `json:"file_types"`
	RequiredOutputSchema Classification[string]           `json:"required_output_schema"`
}

type TaskInput struct {
	Prompt               string
	Context              string
	Tools                []string
	Files                []string
	RequiredOutputSchema string
	TaskFamilyOverride   TaskFamily
}

func IsTaskFamily(value TaskFamily) bool {
	switch value {
	case TaskFamilyGeneralQA, TaskFamilyClassification, TaskFamilyExtraction, TaskFamilyDocumentation,
		TaskFamilySummarization, TaskFamilyCodeGeneration, TaskFamilyCodeReview, TaskFamilyDebugging,
		TaskFamilyArchitectureDesign, TaskFamilyTestGeneration, TaskFamilyErrorRecovery, TaskFamilyDataAnalysis,
		TaskFamilyToolUsePlanning, TaskFamilyKubernetesTroubleshooting:
		return true
	default:
		return false
	}
}

// ClassifyTask uses only fixed local rules; it never calls a model API.
func ClassifyTask(input TaskInput) TaskProfile {
	text := strings.ToLower(input.Prompt + "\n" + input.Context)
	tokens := estimateTokens(input.Prompt + "\n" + input.Context)
	tools := normalized(input.Tools)
	files := fileTypes(input.Files)
	schema := outputSchema(input.RequiredOutputSchema, text)

	profile := TaskProfile{
		SchemaVersion: TaskProfileSchemaVersion, RuleVersion: TaskRuleVersion,
		TaskFamilyVersion: TaskFamilyVersion, DifficultyVersion: DifficultyVersion,
		TokenEstimate:        classified(tokens, 1, "l0:unicode token estimate"),
		ToolDeclarations:     classified(tools, 1, evidenceForList("l0:tool declarations", tools, "l0:no tool declarations")),
		FileTypes:            classified(files, 1, evidenceForList("l0:file types", files, "l0:no file types")),
		RequiredOutputSchema: classified(schema, schemaConfidence(schema), schemaEvidence(schema)),
		ContextSize:          classified(contextSize(tokens), 1, "l0:token estimate"),
	}

	profile.RequiredCapabilities = requiredCapabilities(tools, files, schema)
	profile.Domain = domainFor(text)
	profile.RiskLevel = riskFor(text)
	profile.Seriousness = seriousnessFor(text)
	profile.TaskFamily = familyFor(text, schema, input.TaskFamilyOverride)
	profile.Difficulty = difficultyFor(profile.TaskFamily.Value, profile.ContextSize.Value, profile.RiskLevel.Value)
	profile.VerificationMode = verificationFor(profile.TaskFamily.Value, schema)
	return profile
}

func classified[T any](value T, confidence float64, evidence ...string) Classification[T] {
	return Classification[T]{Value: value, Confidence: confidence, Evidence: evidence}
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (utf8.RuneCountInString(text) + 3) / 4
}

func contextSize(tokens int) ContextSize {
	switch {
	case tokens <= 512:
		return ContextTiny
	case tokens <= 2_000:
		return ContextSmall
	case tokens <= 8_000:
		return ContextMedium
	case tokens <= 32_000:
		return ContextLarge
	default:
		return ContextVeryLarge
	}
}

func normalized(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			seen[value] = true
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func fileTypes(files []string) []string {
	values := make([]string, 0, len(files))
	for _, file := range files {
		name := file[strings.LastIndexAny(file, "/\\")+1:]
		if dot := strings.LastIndexByte(name, '.'); dot > 0 && dot < len(name)-1 {
			extension := strings.ToLower(name[dot:])
			values = append(values, extension)
		}
	}
	return normalized(values)
}

func outputSchema(required, text string) string {
	if strings.TrimSpace(required) != "" || containsAny(text, "json schema", "json格式", "json 格式", "json output", "json输出") {
		return "json_schema"
	}
	return "unknown"
}

func schemaConfidence(schema string) float64 {
	if schema == "unknown" {
		return 0
	}
	return 1
}

func schemaEvidence(schema string) string {
	if schema == "unknown" {
		return "l0:no required output schema"
	}
	return "l0:required output schema"
}

func evidenceForList(source string, values []string, unknown string) string {
	if len(values) == 0 {
		return unknown
	}
	return source
}

func requiredCapabilities(tools, files []string, schema string) Classification[[]Capability] {
	capabilities := []Capability{}
	var evidence []string
	if len(tools) > 0 {
		capabilities = append(capabilities, CapabilityTools)
		evidence = append(evidence, "l0:tool declarations")
	}
	if len(files) > 0 {
		capabilities = append(capabilities, CapabilityFileContext)
		evidence = append(evidence, "l0:file types")
	}
	if schema != "unknown" {
		capabilities = append(capabilities, CapabilityStructuredOutput)
		evidence = append(evidence, "l0:required output schema")
	}
	if len(evidence) == 0 {
		evidence = []string{"l0:no required capabilities"}
	}
	return classified(capabilities, 1, evidence...)
}

func domainFor(text string) Classification[Domain] {
	if containsAny(text, "kubernetes", "kubectl", "k8s", "kubelet", "helm", "pod", "集群") {
		return classified(DomainKubernetes, .95, "l1:kubernetes keyword")
	}
	if containsAny(text, "code", "golang", "python", "typescript", "java", "function", "unit test", "代码", "函数", "编译") {
		return classified(DomainSoftwareEngineering, .9, "l1:software keyword")
	}
	if containsAny(text, "dataset", "csv", "sql", "data analysis", "数据分析") {
		return classified(DomainData, .9, "l1:data keyword")
	}
	return classified(DomainUnknown, 0, "l1:no domain rule")
}

func riskFor(text string) Classification[RiskLevel] {
	if containsAny(text, "medical", "health diagnosis", "legal", "financial advice", "security incident", "生产事故", "医疗", "法律", "金融建议", "安全事故") {
		return classified(RiskHigh, .9, "l1:high-risk keyword")
	}
	return classified(RiskUnknown, 0, "l1:no risk rule")
}

func seriousnessFor(text string) Classification[Seriousness] {
	if containsAny(text, "production outage", "sev", "urgent", "紧急", "线上故障", "生产故障") {
		return classified(SeriousnessHigh, .9, "l1:urgency keyword")
	}
	return classified(SeriousnessUnknown, 0, "l1:no seriousness rule")
}

func familyFor(text, schema string, override TaskFamily) Classification[TaskFamily] {
	if IsTaskFamily(override) {
		return classified(override, 1, "l1:--task-family")
	}
	for _, rule := range []struct {
		family TaskFamily
		terms  []string
	}{
		{TaskFamilyKubernetesTroubleshooting, []string{"kubernetes error", "k8s error", "kubectl", "kubelet", "pod crash", "kubernetes故障", "k8s故障", "集群故障"}},
		{TaskFamilyErrorRecovery, []string{"retry", "fallback", "recover from", "重试", "恢复"}},
		{TaskFamilyArchitectureDesign, []string{"architecture", "system design", "架构设计", "系统设计"}},
		{TaskFamilyCodeReview, []string{"code review", "review this code", "代码审查", "审查代码"}},
		{TaskFamilyTestGeneration, []string{"write tests", "unit test", "test case", "测试用例", "编写测试"}},
		{TaskFamilyDebugging, []string{"debug", "bug", "stack trace", "报错", "调试"}},
		{TaskFamilyCodeGeneration, []string{"implement", "write code", "refactor", "代码实现", "编写代码", "重构"}},
		{TaskFamilyDataAnalysis, []string{"data analysis", "analyze dataset", "数据分析"}},
		{TaskFamilyToolUsePlanning, []string{"tool call", "use tools", "plan tools", "调用工具", "工具规划"}},
		{TaskFamilyDocumentation, []string{"documentation", "readme", "docs", "文档"}},
		{TaskFamilySummarization, []string{"summarize", "summary", "总结", "摘要"}},
		{TaskFamilyClassification, []string{"classify", "categorize", "分类"}},
		{TaskFamilyExtraction, []string{"extract", "提取"}},
	} {
		if containsAny(text, rule.terms...) {
			return classified(rule.family, .9, "l1:keyword:"+rule.terms[0])
		}
	}
	if schema != "unknown" {
		return classified(TaskFamilyExtraction, .75, "l1:structured output")
	}
	return classified(TaskFamilyGeneralQA, .5, "l1:default")
}

func difficultyFor(family TaskFamily, size ContextSize, risk RiskLevel) Classification[Difficulty] {
	if risk == RiskHigh {
		return classified(DifficultyFive, .8, "l1:high-risk task")
	}
	if size == ContextLarge || size == ContextVeryLarge {
		return classified(DifficultyFour, .8, "l0:large context")
	}
	switch family {
	case TaskFamilyArchitectureDesign, TaskFamilyKubernetesTroubleshooting:
		return classified(DifficultyFour, .8, "l1:complex task family")
	case TaskFamilyCodeGeneration, TaskFamilyCodeReview, TaskFamilyDebugging, TaskFamilyTestGeneration, TaskFamilyToolUsePlanning, TaskFamilyErrorRecovery:
		return classified(DifficultyThree, .8, "l1:task family")
	case TaskFamilyClassification, TaskFamilyExtraction, TaskFamilyDocumentation, TaskFamilySummarization, TaskFamilyDataAnalysis:
		return classified(DifficultyTwo, .8, "l1:task family")
	default:
		return classified(DifficultyOne, .5, "l1:default")
	}
}

func verificationFor(family TaskFamily, schema string) Classification[VerificationMode] {
	if schema != "unknown" {
		return classified(VerificationSchema, 1, "l0:required output schema")
	}
	switch family {
	case TaskFamilyCodeGeneration, TaskFamilyCodeReview, TaskFamilyDebugging, TaskFamilyTestGeneration, TaskFamilyKubernetesTroubleshooting:
		return classified(VerificationTests, .8, "l1:task family")
	default:
		return classified(VerificationUnknown, 0, "l1:no verification rule")
	}
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}
