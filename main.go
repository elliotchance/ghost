package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"flag"
	"strings"
)

var fset *token.FileSet
var currentFunction *ast.FuncDecl
var hasErrors bool
var (
	optionIgnoreTests       bool
	optionMaxLineComplexity int
	optionNeverFail         bool
)

func main() {
	flag.BoolVar(&optionNeverFail, "never-fail", false, "Always exit with 0.")
	flag.BoolVar(&optionIgnoreTests, "ignore-tests", false, "Ignore test files.")
	flag.IntVar(&optionMaxLineComplexity, "max-line-complexity", 5,
		"The maximum allowed line complexity.")
	flag.Parse()

	for _, currentFile := range os.Args[1:] {
		if !strings.HasSuffix(currentFile, ".go") {
			continue
		}

		if optionIgnoreTests && strings.HasSuffix(currentFile, "_test.go") {
			continue
		}

		fset = token.NewFileSet()
		node, err := parser.ParseFile(fset, currentFile, nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			checkFunction(fn)
		}
	}

	if hasErrors && !optionNeverFail {
		os.Exit(1)
	}
}

func checkFunction(fn *ast.FuncDecl) {
	currentFunction = fn

	for _, stmt := range fn.Body.List {
		checkLine(stmt)
	}
}

func checkLine(line ast.Stmt) bool {
	complexity := LineComplexity(line)

	if complexity > optionMaxLineComplexity {
		printLine(complexity, line)
	}

	return complexity <= optionMaxLineComplexity
}

func LineComplexity(line ast.Stmt) int {
	switch n := line.(type) {
	case nil:
		return 0

	case *ast.AssignStmt:
		return exprsComplexity(n.Rhs)

	case *ast.ExprStmt:
		return exprComplexity(n.X)

	case *ast.ReturnStmt:
		if len(n.Results) == 0 {
			return 0
		}

		return listComplexity(n.Results)

	case *ast.IfStmt:
		for _, l := range n.Body.List {
			LineComplexity(l)
		}

		return exprComplexity(n.Cond)

	case *ast.ForStmt:
		if n.Cond == nil {
			return 0
		}

		for _, l := range n.Body.List {
			LineComplexity(l)
		}

		return 1 + exprComplexity(n.Cond)

	case *ast.SwitchStmt:
		if n.Tag == nil {
			return 0
		}

		for _, l := range n.Body.List {
			LineComplexity(l)
		}

		return 1 + exprComplexity(n.Tag)

	case *ast.DeferStmt:
		return exprComplexity(n.Call.Fun)

	case *ast.TypeSwitchStmt:
		for _, l := range n.Body.List {
			LineComplexity(l)
		}

		return 1

	case *ast.RangeStmt:
		return exprComplexity(n.X)

	case *ast.DeclStmt:
		specs := n.Decl.(*ast.GenDecl).Specs

		total := 0
		for _, spec := range specs {
			total += exprsComplexity(spec.(*ast.ValueSpec).Values)
		}

		return total

	case *ast.CaseClause:
		for _, l := range n.Body {
			LineComplexity(l)
		}

		return listComplexity(n.List)

	case *ast.IncDecStmt, *ast.BranchStmt:
		return 1

	default:
		printLine(-1, line)
		panic(n)
	}
}

func exprsComplexity(exprs []ast.Expr) (total int) {
	for _, expr := range exprs {
		total += exprComplexity(expr)
	}

	return
}

func listComplexity(exprs []ast.Expr) int {
	total := 1

	for _, expr := range exprs {
		complexity := exprComplexity(expr) - 1
		if complexity < 0 {
			complexity = 0
		}

		total += complexity
	}

	return total
}

func exprComplexity(expr ast.Expr) int {
	switch e := expr.(type) {
	case nil, *ast.BasicLit, *ast.Ident, *ast.ArrayType:
		return 0

	case *ast.UnaryExpr, *ast.TypeAssertExpr:
		return 1

	case *ast.StarExpr:
		return exprComplexity(e.X)

	case *ast.CallExpr:
		return 1 + exprsComplexity(e.Args)

	case *ast.BinaryExpr:
		left := exprComplexity(e.X)
		right := exprComplexity(e.Y)

		if _, ok := e.X.(*ast.CallExpr); ok {
			left++
		} else if _, ok := e.Y.(*ast.CallExpr); ok {
			right++
		}

		if left+right == 0 {
			return 1
		}

		return left + right

	case *ast.CompositeLit:
		return listComplexity(e.Elts)

	case *ast.SelectorExpr:
		return 1

	case *ast.IndexExpr:
		return 1 + exprComplexity(e.Index)

	case *ast.KeyValueExpr:
		return exprComplexity(e.Value)

	case *ast.ParenExpr:
		return exprComplexity(e.X)

	case *ast.FuncLit:
		for _, l := range e.Body.List {
			LineComplexity(l)
		}

		return 1

	case *ast.SliceExpr:
		complexity := exprsComplexity([]ast.Expr{e.Low, e.High, e.Max})

		return 1 + complexity

	case *ast.MapType:
		return 0

	default:
		panic(e)
	}
}

func printLine(complexity int, line ast.Stmt) {
	hasErrors = true
	pos := fset.Position(line.Pos())
	functionName := currentFunction.Name.Name
	fmt.Printf("%s:%d: complexity is %d (in %s)\n", pos.Filename, pos.Line,
		complexity, functionName)
}
