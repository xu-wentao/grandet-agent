package application

import "github.com/xu-wentao/grandet-agent/internal/domain"

type TaskInput struct {
	Prompt               string
	Context              string
	Tools                []string
	Files                []string
	RequiredOutputSchema string
	TaskFamilyOverride   string
}

func ClassifyTask(input TaskInput) domain.TaskProfile {
	return domain.ClassifyTask(domain.TaskInput{
		Prompt: input.Prompt, Context: input.Context, Tools: input.Tools, Files: input.Files,
		RequiredOutputSchema: input.RequiredOutputSchema, TaskFamilyOverride: domain.TaskFamily(input.TaskFamilyOverride),
	})
}

func IsTaskFamily(value string) bool { return domain.IsTaskFamily(domain.TaskFamily(value)) }
