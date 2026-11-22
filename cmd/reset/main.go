// Command reset сканирует пакеты Go, начиная с корневой директории, и для
// каждой структуры, помеченной комментарием // generate:reset, генерирует
// метод Reset в файле reset.gen.go этого же пакета.
//
// Запуск из корня проекта:
//
//	go run ./cmd/reset [dir]
//
// Правила сброса: примитивы приводятся к нулевым значениям; слайсы
// обрезаются до нулевой длины ([:0]) с сохранением ёмкости; мапы
// очищаются встроенным clear; вложенные структуры, обладающие методом
// Reset, вызывают его; не-nil указатели сбрасывают своё значение по тем
// же правилам.
package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const (
	marker      = "generate:reset"
	genFileName = "reset.gen.go"
)

var basicZeros = map[string]string{
	"bool":       "false",
	"string":     `""`,
	"error":      "nil",
	"any":        "nil",
	"int":        "0",
	"int8":       "0",
	"int16":      "0",
	"int32":      "0",
	"int64":      "0",
	"uint":       "0",
	"uint8":      "0",
	"uint16":     "0",
	"uint32":     "0",
	"uint64":     "0",
	"uintptr":    "0",
	"byte":       "0",
	"rune":       "0",
	"float32":    "0",
	"float64":    "0",
	"complex64":  "0",
	"complex128": "0",
}

func main() {
	run()
}

func run() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	count, err := generateTree(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reset generator:", err)
		os.Exit(1)
	}

	fmt.Printf("reset.gen.go files generated: %d\n", count)
}

type importInfo struct {
	alias string
	path  string
}

type packageGen struct {
	name       string
	structs    map[string]*ast.StructType
	typeParams map[string]*ast.FieldList
	resettable map[string]bool
	imports    map[string]importInfo
	marked     []string
}

func generateTree(root string) (int, error) {
	fset := token.NewFileSet()
	generated := 0

	walkErr := filepath.WalkDir(root, func(dirPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if dirPath != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor") {
			return fs.SkipDir
		}

		pkgName, files, err := parseDir(fset, dirPath)
		if err != nil {
			return fmt.Errorf("parse dir %s: %w", dirPath, err)
		}
		if len(files) == 0 {
			return nil
		}

		written, err := generatePackage(dirPath, pkgName, files)
		if err != nil {
			return err
		}
		if written {
			generated++
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}

	return generated, nil
}

func parseDir(fset *token.FileSet, dir string) (string, []*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}

	pkgName := ""
	var files []*ast.File

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return "", nil, err
		}

		if pkgName == "" {
			pkgName = file.Name.Name
		}
		if file.Name.Name != pkgName {
			continue
		}
		files = append(files, file)
	}

	return pkgName, files, nil
}

func generatePackage(dir, pkgName string, files []*ast.File) (bool, error) {
	gen := collectPackage(pkgName, files)
	if len(gen.marked) == 0 {
		return false, nil
	}

	src, err := renderFile(gen)
	if err != nil {
		return false, fmt.Errorf("render package %s: %w", gen.name, err)
	}

	out := filepath.Join(dir, genFileName)
	if err := os.WriteFile(out, src, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", out, err)
	}
	return true, nil
}

func collectPackage(pkgName string, files []*ast.File) *packageGen {
	gen := &packageGen{
		name:       pkgName,
		structs:    make(map[string]*ast.StructType),
		typeParams: make(map[string]*ast.FieldList),
		resettable: make(map[string]bool),
		imports:    make(map[string]importInfo),
	}

	for _, file := range files {
		collectImports(file, gen.imports)

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					collectTypes(d, gen)
				}
			case *ast.FuncDecl:
				if d.Recv != nil && d.Name.Name == "Reset" && d.Type.Params.NumFields() == 0 {
					if typeName := receiverTypeName(d.Recv); typeName != "" {
						gen.resettable[typeName] = true
					}
				}
			}
		}
	}

	sort.Strings(gen.marked)
	for _, name := range gen.marked {
		gen.resettable[name] = true
	}
	return gen
}

func collectTypes(d *ast.GenDecl, gen *packageGen) {
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			continue
		}

		gen.structs[ts.Name.Name] = st
		gen.typeParams[ts.Name.Name] = ts.TypeParams

		if hasMarker(ts.Doc) || (len(d.Specs) == 1 && hasMarker(d.Doc)) {
			gen.marked = append(gen.marked, ts.Name.Name)
		}
	}
}

func collectImports(file *ast.File, imports map[string]importInfo) {
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		qualifier := path.Base(importPath)
		info := importInfo{path: importPath}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			qualifier = imp.Name.Name
			info.alias = imp.Name.Name
		}

		if _, exists := imports[qualifier]; !exists {
			imports[qualifier] = info
		}
	}
}

func hasMarker(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		if strings.TrimSpace(strings.TrimPrefix(c.Text, "//")) == marker {
			return true
		}
	}
	return false
}

func receiverTypeName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}

	expr := recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

func renderFile(gen *packageGen) ([]byte, error) {
	used := make(map[string]bool)

	var body strings.Builder
	for _, name := range gen.marked {
		body.WriteString("\n")
		body.WriteString(renderReset(name, gen, used))
	}

	var out strings.Builder
	out.WriteString("// Code generated by cmd/reset. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n", gen.name)

	qualifiers := make([]string, 0, len(used))
	for qualifier := range used {
		qualifiers = append(qualifiers, qualifier)
	}
	sort.Strings(qualifiers)

	if len(qualifiers) > 0 {
		out.WriteString("\nimport (\n")
		for _, qualifier := range qualifiers {
			info := gen.imports[qualifier]
			if info.alias != "" {
				fmt.Fprintf(&out, "\t%s %q\n", info.alias, info.path)
			} else {
				fmt.Fprintf(&out, "\t%q\n", info.path)
			}
		}
		out.WriteString(")\n")
	}

	out.WriteString(body.String())

	src, err := format.Source([]byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return src, nil
}

func renderReset(name string, gen *packageGen, used map[string]bool) string {
	recv := receiverName(name)
	st := gen.structs[name]

	var b strings.Builder
	fmt.Fprintf(&b, "func (%s *%s%s) Reset() {\n", recv, name, typeParamsString(gen.typeParams[name]))
	fmt.Fprintf(&b, "\tif %s == nil {\n\t\treturn\n\t}\n", recv)

	for _, field := range st.Fields.List {
		for _, fieldName := range fieldNames(field) {
			if fieldName == "" || fieldName == "_" {
				continue
			}
			for _, stmt := range fieldStatements(recv, fieldName, field.Type, gen, used) {
				fmt.Fprintf(&b, "\n\t%s\n", stmt)
			}
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func fieldNames(field *ast.Field) []string {
	if len(field.Names) > 0 {
		names := make([]string, 0, len(field.Names))
		for _, id := range field.Names {
			names = append(names, id.Name)
		}
		return names
	}
	return []string{embeddedName(field.Type)}
}

func embeddedName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(t.X)
	case *ast.IndexListExpr:
		return embeddedName(t.X)
	default:
		return ""
	}
}

func fieldStatements(recv, name string, expr ast.Expr, gen *packageGen, used map[string]bool) []string {
	sel := recv + "." + name

	switch t := expr.(type) {
	case *ast.ArrayType:
		if t.Len == nil {
			return []string{fmt.Sprintf("%s = %s[:0]", sel, sel)}
		}
		return []string{zeroAssign(sel, expr, gen, used)}
	case *ast.MapType:
		return []string{fmt.Sprintf("clear(%s)", sel)}
	case *ast.InterfaceType, *ast.ChanType, *ast.FuncType:
		return []string{sel + " = nil"}
	case *ast.StarExpr:
		return pointerStatements(sel, t.X, gen, used)
	case *ast.Ident:
		return identStatements(sel, t, gen)
	default:
		return []string{zeroAssign(sel, expr, gen, used)}
	}
}

func identStatements(sel string, id *ast.Ident, gen *packageGen) []string {
	if zero, ok := basicZeros[id.Name]; ok {
		return []string{fmt.Sprintf("%s = %s", sel, zero)}
	}
	if gen.resettable[id.Name] {
		return []string{fmt.Sprintf("%s.Reset()", sel)}
	}
	return []string{fmt.Sprintf("%s = *new(%s)", sel, id.Name)}
}

func pointerStatements(sel string, x ast.Expr, gen *packageGen, used map[string]bool) []string {
	return []string{fmt.Sprintf("if %s != nil {\n\t\t%s\n\t}", sel, derefStatement(sel, x, gen, used))}
}

func derefStatement(sel string, x ast.Expr, gen *packageGen, used map[string]bool) string {
	switch t := x.(type) {
	case *ast.Ident:
		if zero, ok := basicZeros[t.Name]; ok {
			return fmt.Sprintf("*%s = %s", sel, zero)
		}
		if gen.resettable[t.Name] {
			return sel + ".Reset()"
		}
		return fmt.Sprintf("*%s = *new(%s)", sel, t.Name)
	case *ast.ArrayType:
		if t.Len == nil {
			return fmt.Sprintf("*%s = (*%s)[:0]", sel, sel)
		}
		return zeroDeref(sel, x, gen, used)
	case *ast.MapType:
		return fmt.Sprintf("clear(*%s)", sel)
	default:
		return zeroDeref(sel, x, gen, used)
	}
}

func zeroAssign(sel string, expr ast.Expr, gen *packageGen, used map[string]bool) string {
	markUsedImports(expr, gen, used)
	return fmt.Sprintf("%s = *new(%s)", sel, types.ExprString(expr))
}

func zeroDeref(sel string, expr ast.Expr, gen *packageGen, used map[string]bool) string {
	markUsedImports(expr, gen, used)
	return fmt.Sprintf("*%s = *new(%s)", sel, types.ExprString(expr))
}

func markUsedImports(expr ast.Expr, gen *packageGen, used map[string]bool) {
	ast.Inspect(expr, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := s.X.(*ast.Ident); ok {
			if _, known := gen.imports[id.Name]; known {
				used[id.Name] = true
			}
		}
		return true
	})
}

func receiverName(typeName string) string {
	first := []rune(typeName)[0]
	return string(unicode.ToLower(first))
}

func typeParamsString(tparams *ast.FieldList) string {
	if tparams == nil || len(tparams.List) == 0 {
		return ""
	}

	parts := make([]string, 0, len(tparams.List))
	for _, field := range tparams.List {
		constraint := types.ExprString(field.Type)
		for _, id := range field.Names {
			parts = append(parts, id.Name+" "+constraint)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
