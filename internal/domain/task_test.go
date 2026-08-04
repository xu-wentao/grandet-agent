package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestClassifyTaskFixtures(t *testing.T) {
	longContext := strings.Repeat("context ", 20_000)
	for _, test := range []struct {
		name       string
		input      TaskInput
		family     TaskFamily
		difficulty Difficulty
		domain     Domain
		context    ContextSize
		capability Capability
		verify     VerificationMode
	}{
		{"ambiguous", TaskInput{Prompt: "Can you help me?"}, TaskFamilyGeneralQA, DifficultyOne, DomainUnknown, ContextTiny, "", VerificationUnknown},
		{"long context", TaskInput{Prompt: "Explain this", Context: longContext}, TaskFamilyGeneralQA, DifficultyFour, DomainUnknown, ContextVeryLarge, "", VerificationUnknown},
		{"structured extraction", TaskInput{Prompt: "Extract the invoice fields", Tools: []string{"fetch"}, Files: []string{"invoice.PDF"}, RequiredOutputSchema: "invoice.json"}, TaskFamilyExtraction, DifficultyTwo, DomainUnknown, ContextTiny, CapabilityStructuredOutput, VerificationSchema},
		{"english coding", TaskInput{Prompt: "Implement a Go function and write unit tests"}, TaskFamilyTestGeneration, DifficultyThree, DomainSoftwareEngineering, ContextTiny, "", VerificationTests},
		{"chinese kubernetes", TaskInput{Prompt: "诊断 Kubernetes 集群故障：Pod CrashLoopBackOff"}, TaskFamilyKubernetesTroubleshooting, DifficultyFour, DomainKubernetes, ContextTiny, "", VerificationTests},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := ClassifyTask(test.input)
			if profile.TaskFamily.Value != test.family || profile.Difficulty.Value != test.difficulty || profile.Domain.Value != test.domain || profile.ContextSize.Value != test.context || profile.VerificationMode.Value != test.verify {
				t.Fatalf("profile = %#v", profile)
			}
			if test.capability != "" && !hasCapability(profile.RequiredCapabilities.Value, test.capability) {
				t.Fatalf("capabilities = %#v", profile.RequiredCapabilities)
			}
			for _, evidence := range [][]string{profile.TaskFamily.Evidence, profile.Difficulty.Evidence, profile.Domain.Evidence, profile.RiskLevel.Evidence, profile.Seriousness.Evidence, profile.ContextSize.Evidence, profile.RequiredCapabilities.Evidence, profile.VerificationMode.Evidence, profile.TokenEstimate.Evidence, profile.ToolDeclarations.Evidence, profile.FileTypes.Evidence, profile.RequiredOutputSchema.Evidence} {
				if len(evidence) == 0 {
					t.Fatal("classification field has no evidence")
				}
			}
		})
	}
}

func TestClassifyTaskIsDeterministic(t *testing.T) {
	input := TaskInput{Prompt: "Extract JSON from this Go file", Tools: []string{"HTTP", "http"}, Files: []string{"main.go", "README.md"}, RequiredOutputSchema: "result.json"}
	first := ClassifyTask(input)
	if second := ClassifyTask(input); !reflect.DeepEqual(first, second) {
		t.Fatalf("first = %#v\nsecond = %#v", first, second)
	}
	if first.TaskFamilyVersion != TaskFamilyVersion || first.DifficultyVersion != DifficultyVersion || first.RuleVersion != TaskRuleVersion {
		t.Fatalf("versions = %#v", first)
	}
}

func hasCapability(values []Capability, wanted Capability) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
