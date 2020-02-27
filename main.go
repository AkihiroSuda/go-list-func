package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"os"
	"strings"
	"unicode"

	"golang.org/x/tools/go/loader"
)

func main() {
	var buildTags string
	var includeTests bool
	var verbose bool
	flag.StringVar(&buildTags, "tags", "", "build tags")
	flag.BoolVar(&includeTests, "include-tests", false, "include tests")
	flag.BoolVar(&verbose, "verbose", false, "verbose")
	flag.Parse()
	prog, _, err := loadProgram(parseBuildTags(buildTags), flag.Args(), includeTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err = printFuncsInProgram(prog, verbose); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func parseBuildTags(tags string) []string {
	var result []string
	split := strings.Split(tags, ",")
	for _, s := range split {
		result = append(result, strings.TrimSpace(s))
	}
	return result
}

func loadProgram(tags, args []string, includeTests bool) (*loader.Program, []string, error) {
	var conf loader.Config
	conf.Build = &build.Default
	conf.Build.BuildTags = append(conf.Build.BuildTags, tags...)
	rest, err := conf.FromArgs(args, includeTests)
	if err != nil {
		return nil, rest, err
	}
	prog, err := conf.Load()
	return prog, rest, err
}

func printFuncsInProgram(prog *loader.Program, verbose bool) error {
	for _, pkgInfo := range prog.InitialPackages() {
		for _, file := range pkgInfo.Files {
			if err := printFuncsInFile(file, verbose); err != nil {
				return err
			}
		}
	}
	return nil
}

func printFuncsInFile(file *ast.File, verbose bool) error {
	for _, xdecl := range file.Decls {
		switch decl := xdecl.(type) {
		case *ast.FuncDecl:
			if exported(decl) {
				if verbose {
					fmt.Println(formatFuncDecl(decl))
				} else {
					fmt.Println(decl.Name.Name)
				}
			}
		}
	}
	return nil
}

func exported(decl *ast.FuncDecl) bool {
	isUpper0 := func(s string) bool {
		if strings.HasPrefix(s, "*") {
			return unicode.IsUpper([]rune(s)[1])
		}
		return unicode.IsUpper([]rune(s)[0])
	}
	if decl.Recv != nil {
		if len(decl.Recv.List) != 1 {
			panic(fmt.Errorf("strange receiver for %s: %#v", decl.Name.Name, decl.Recv))
		}
		field := decl.Recv.List[0]
		return isUpper0(formatType(field.Type)) && isUpper0(decl.Name.Name)
	}
	return isUpper0(decl.Name.Name)
}

func formatType(typ ast.Expr) string {
	switch t := typ.(type) {
	case nil:
		return ""
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", formatType(t.X), t.Sel.Name)
	case *ast.StarExpr:
		return fmt.Sprintf("*%s", formatType(t.X))
	case *ast.ArrayType:
		return fmt.Sprintf("[%s]%s", formatType(t.Len), formatType(t.Elt))
	case *ast.Ellipsis:
		return formatType(t.Elt)
	case *ast.FuncType:
		return fmt.Sprintf("func(%s)%s", formatFuncParams(t.Params), formatFuncResults(t.Results))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatType(t.Key), formatType(t.Value))
	case *ast.ChanType:
		// FIXME
		panic(fmt.Errorf("unsupported chan type %#v", t))
	case *ast.BasicLit:
		return t.Value
	default:
		panic(fmt.Errorf("unsupported type %#v", t))
	}
}

func formatFields(fields *ast.FieldList) string {
	s := ""
	for i, field := range fields.List {
		for j, name := range field.Names {
			s += name.Name
			if j != len(field.Names)-1 {
				s += ","
			}
			s += " "
		}
		s += formatType(field.Type)
		if i != len(fields.List)-1 {
			s += ", "
		}
	}
	return s
}

func formatFuncParams(fields *ast.FieldList) string {
	return formatFields(fields)
}

func formatFuncResults(fields *ast.FieldList) string {
	s := ""
	if fields != nil {
		s += " "
		if len(fields.List) > 1 {
			s += "("
		}
		s += formatFields(fields)
		if len(fields.List) > 1 {
			s += ")"
		}
	}
	return s
}

func formatFuncDecl(decl *ast.FuncDecl) string {
	s := "func "
	if decl.Recv != nil {
		if len(decl.Recv.List) != 1 {
			panic(fmt.Errorf("strange receiver for %s: %#v", decl.Name.Name, decl.Recv))
		}
		field := decl.Recv.List[0]
		if len(field.Names) == 0 {
			// function definition in interface (ignore)
			return ""
		} else if len(field.Names) != 1 {
			panic(fmt.Errorf("strange receiver field for %s: %#v", decl.Name.Name, field))
		}
		s += fmt.Sprintf("(%s %s) ", field.Names[0], formatType(field.Type))
	}
	s += fmt.Sprintf("%s(%s)", decl.Name.Name, formatFuncParams(decl.Type.Params))
	s += formatFuncResults(decl.Type.Results)
	return s
}
