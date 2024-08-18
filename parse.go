package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"strings"
)

type Func struct {
	FullDescriptions         []string
	FunctionDescriptions     []FunctionDescription
	TestFunctionDescriptions []FunctionDescription
}

type FunctionDescription struct {
	Name           string `json:"name"`
	Doc            string `json:"doc"`
	Package        string `json:"package"`
	IsTestFunction bool   `json:"is_test_function"`
}

type Param struct {
	FilePath    string
	FileName    string
	IncludeBody bool
}

func (f *Func) ParseFunctions(p Param) {
	code, err := readFile(p.FilePath)
	if err != nil {
		log.Printf("Error reading file %s: %v", p.FilePath, err)
		return
	}

	file, err := parseCode(p.FileName, code)
	if err != nil {
		log.Printf("Error parsing file %s: %v", p.FileName, err)
		return
	}

	description, funcDescriptions, testFuncDescriptions := buildFileDescription(p, file, code)
	f.FullDescriptions = append(f.FullDescriptions, description)
	f.FunctionDescriptions = append(f.FunctionDescriptions, funcDescriptions...)
	f.TestFunctionDescriptions = append(f.TestFunctionDescriptions, testFuncDescriptions...)
}

func (f *Func) Print() {
	for _, desc := range f.FullDescriptions {
		fmt.Println(desc)
	}
}

func readFile(filePath string) (string, error) {
	codeFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer codeFile.Close()

	srcbuf, err := io.ReadAll(codeFile)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(srcbuf), nil
}

func parseCode(fileName, code string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, fileName, code, parser.ParseComments)
}

func buildFileDescription(p Param, file *ast.File, code string) (string, []FunctionDescription, []FunctionDescription) {
	var sb strings.Builder
	var funcDescriptions, testFuncDescriptions []FunctionDescription

	isTestFile := strings.Contains(p.FileName, "_test")
	writeFileHeader(&sb, p, file, isTestFile)

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			funcStr := describeFunctionDeclaration(&sb, fn, code, p.IncludeBody)
			funcDesc := FunctionDescription{
				Name:           fn.Name.Name,
				Doc:            funcStr,
				Package:        file.Name.Name,
				IsTestFunction: isTestFile,
			}
			if isTestFile {
				testFuncDescriptions = append(testFuncDescriptions, funcDesc)
			} else {
				funcDescriptions = append(funcDescriptions, funcDesc)
			}
		}
		return true
	})

	writeFileFooter(&sb, p, isTestFile)
	return sb.String(), funcDescriptions, testFuncDescriptions
}

func writeFileHeader(sb *strings.Builder, p Param, file *ast.File, isTestFile bool) {
	fileType := "go"
	if isTestFile {
		fileType += " test"
	}
	sb.WriteString(fmt.Sprintf("##Start of %s file %s\n", fileType, p.FilePath))
	sb.WriteString(fmt.Sprintf("###File path: %s\n", p.FilePath))
	sb.WriteString(fmt.Sprintf("###File name: %s\n", p.FileName))
	sb.WriteString(fmt.Sprintf("##Package name: %s\n", file.Name.Name))
	sb.WriteString(fmt.Sprintf("##%s\n", strings.Title(fileType)+" Functions"))
}

func writeFileFooter(sb *strings.Builder, p Param, isTestFile bool) {
	fileType := "go"
	if isTestFile {
		fileType += " test"
	}
	sb.WriteString(fmt.Sprintf("----- End of %s file %s -------\n", fileType, p.FilePath))
}

func describeFunctionDeclaration(funcSb *strings.Builder, fn *ast.FuncDecl, code string, includeBody bool) string {
	var sb strings.Builder
	writeComments(&sb, fn.Doc)
	sb.WriteString(fmt.Sprintf("## %s\n\n", fn.Name.Name))

	if fn.Recv != nil {
		sb.WriteString(fmt.Sprintf("## Receiver\n\n%s\n\n", fields(*fn.Recv)))
	}

	writeParameters(&sb, fn.Type.Params)
	writeResults(&sb, fn.Type.Results)
	writeFunctionCalls(&sb, fn, code)

	if includeBody {
		writeFunctionBody(&sb, fn, code)
	}

	sb.WriteString(fmt.Sprintf("`###End of function with name %s  ###`\n\n", fn.Name.Name))
	funcSb.WriteString(sb.String())
	return sb.String()
}

func writeComments(sb *strings.Builder, doc *ast.CommentGroup) {
	if doc != nil {
		for _, c := range doc.List {
			sb.WriteString(c.Text + "\n")
		}
	}
}

func writeParameters(sb *strings.Builder, params *ast.FieldList) {
	if params != nil {
		sb.WriteString("##Parameters " + fields(*params) + "\n")
	}
}

func writeResults(sb *strings.Builder, results *ast.FieldList) {
	if results != nil {
		sb.WriteString("##Return " + fields(*results) + "\n")
	}
}

func writeFunctionCalls(sb *strings.Builder, fn *ast.FuncDecl, code string) {
	sb.WriteString("## Function calls from other packages\n\n")
	sb.WriteString("```go\n")
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			sb.WriteString("  " + code[call.Pos()-1:call.End()-1] + "\n")
		}
		return true
	})
	sb.WriteString("```\n")
}

func writeFunctionBody(sb *strings.Builder, fn *ast.FuncDecl, code string) {
	sb.WriteString(fmt.Sprintf("####Function Body of function %s\n\n", fn.Name.Name))
	sb.WriteString("```go\n")
	sb.WriteString(code[fn.Pos()-1 : fn.End()-1])
	sb.WriteString("```\n")
}

func expr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return fmt.Sprintf("*%v", expr(x.X))
	case *ast.Ident:
		return x.Name
	case *ast.ArrayType:
		if x.Len != nil {
			return fmt.Sprintf("[%s]%s", expr(x.Len), expr(x.Elt))
		}
		return fmt.Sprintf("[]%s", expr(x.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", expr(x.Key), expr(x.Value))
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", expr(x.X), expr(x.Sel))
	default:
		log.Printf("Unknown type: %T\n", x)
		return ""
	}
}

func fields(fl ast.FieldList) string {
	var parts []string
	for _, f := range fl.List {
		names := make([]string, len(f.Names))
		for i, n := range f.Names {
			names[i] = n.Name
		}
		part := fmt.Sprintf("%s %s", strings.Join(names, ", "), expr(f.Type))
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
