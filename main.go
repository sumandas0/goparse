package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

type ProjectProcessor struct {
	ProjectPath string
	OutputPath  string
}

func main() {
	app := createCliApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func createCliApp() *cli.App {
	return &cli.App{
		Name:   "parse",
		Usage:  "Parse a go project and generate a json file with all functions and test functions",
		Flags:  createFlags(),
		Action: runApp,
	}
}

func createFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "project",
			Usage:    "The path to the go project",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "output",
			Usage:    "The path to the output directory",
			Required: true,
		},
	}
}

func runApp(context *cli.Context) error {
	processor := ProjectProcessor{
		ProjectPath: context.String("project"),
		OutputPath:  context.String("output"),
	}
	return processor.Process()
}

func (p *ProjectProcessor) Process() error {
	if err := p.validatePaths(); err != nil {
		return err
	}

	goFiles, err := p.findGoFiles()
	if err != nil {
		return fmt.Errorf("failed to find Go files: %w", err)
	}

	funcDescriptions := parseFunctions(goFiles)
	if err := p.writeOutputFiles(funcDescriptions); err != nil {
		return err
	}

	return nil
}

func (p *ProjectProcessor) validatePaths() error {
	if _, err := os.Stat(p.ProjectPath); os.IsNotExist(err) {
		return fmt.Errorf("project path does not exist: %v", err)
	}

	if err := os.MkdirAll(p.OutputPath, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	return nil
}

func (p *ProjectProcessor) findGoFiles() ([]string, error) {
	var goFiles []string

	err := filepath.Walk(p.ProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") && !strings.Contains(info.Name(), "generated") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}

	return goFiles, nil
}

func parseFunctions(goFiles []string) Func {
	funcDescriptions := Func{}
	for _, goFile := range goFiles {
		param := Param{
			FilePath:    goFile,
			FileName:    filepath.Base(goFile),
			IncludeBody: false,
		}
		funcDescriptions.ParseFunctions(param)
	}
	return funcDescriptions
}

func (p *ProjectProcessor) writeOutputFiles(funcDescriptions Func) error {
	allDescriptions := combineDescriptions(funcDescriptions)
	if err := p.writeToFile(allDescriptions, "all_function_descriptions.txt"); err != nil {
		return fmt.Errorf("failed to write descriptions to file: %w", err)
	}

	if err := p.writeJSONFile(funcDescriptions.testFunctionDescriptions, "test_functions.json"); err != nil {
		return fmt.Errorf("failed to write test functions to file: %w", err)
	}

	if err := p.writeJSONFile(funcDescriptions.functionDescriptions, "functions.json"); err != nil {
		return fmt.Errorf("failed to write functions to file: %w", err)
	}

	return nil
}

func combineDescriptions(funcDescriptions Func) string {
	var allDescriptions strings.Builder
	allDescriptions.WriteString("#### This is detailed description of all functions in the project its references\n")
	for _, desc := range funcDescriptions.FullDescriptions {
		allDescriptions.WriteString(desc)
	}
	return allDescriptions.String()
}

func (p *ProjectProcessor) writeToFile(content, filename string) error {
	fullPath := filepath.Join(p.OutputPath, filename)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}(file)

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (p *ProjectProcessor) writeJSONFile(data interface{}, filename string) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return p.writeToFile(string(b), filename)
}
