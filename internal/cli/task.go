package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xu-wentao/grandet-agent/internal/application"
)

func runTask(args []string) error {
	if len(args) == 0 || args[0] != "classify" {
		return fmt.Errorf("task requires classify")
	}
	return runTaskClassify(args[1:])
}

func runTaskClassify(args []string) error {
	args, err := intersperseTaskFlags(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("task classify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var taskFamily, schema string
	var tools, files stringList
	fs.StringVar(&taskFamily, "task-family", "", "Explicit task family")
	fs.StringVar(&schema, "schema", "", "Required output schema")
	fs.Var(&tools, "tool", "Required tool declaration (repeatable)")
	fs.Var(&files, "context", "Context file (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prompt := strings.Join(fs.Args(), " ")
	if prompt == "" {
		return fmt.Errorf("task classify requires a prompt")
	}
	if taskFamily != "" && !application.IsTaskFamily(taskFamily) {
		return fmt.Errorf("unknown task family %q", taskFamily)
	}
	context, err := readContextFiles(files)
	if err != nil {
		return err
	}
	profile := application.ClassifyTask(application.TaskInput{
		Prompt: prompt, Context: context, Tools: tools, Files: files,
		RequiredOutputSchema: schema, TaskFamilyOverride: taskFamily,
	})
	output, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task profile: %w", err)
	}
	fmt.Println(string(output))
	return nil
}

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func readContextFiles(files []string) (string, error) {
	var context strings.Builder
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read context %s: %w", file, err)
		}
		context.Write(contents)
	}
	return context.String(), nil
}

func intersperseTaskFlags(args []string) ([]string, error) {
	withValue := map[string]bool{"--task-family": true, "--schema": true, "--tool": true, "--context": true}
	var flags, prompt []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		name := argument
		if equal := strings.IndexByte(argument, '='); equal >= 0 {
			name = argument[:equal]
		}
		if !withValue[name] {
			prompt = append(prompt, argument)
			continue
		}
		flags = append(flags, argument)
		if !strings.Contains(argument, "=") {
			if index+1 == len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", name)
			}
			index++
			flags = append(flags, args[index])
		}
	}
	return append(flags, prompt...), nil
}
