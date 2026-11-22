package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// osExitAnalyzer — собственный анализатор multichecker: запрещает прямой
// вызов os.Exit в функции main пакета main. Подробности — в документации
// пакета (doc.go).
var osExitAnalyzer = &analysis.Analyzer{
	Name: "osexit",
	Doc:  "forbids direct os.Exit calls in func main of package main",
	Run:  runOsExit,
}

func runOsExit(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Exit" {
					return true
				}

				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}

				pkgName, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
				if !ok || pkgName.Imported().Path() != "os" {
					return true
				}

				pass.Reportf(call.Pos(), "direct os.Exit call in func main is forbidden; move the logic to a separate function")
				return true
			})
		}
	}

	return nil, nil
}
