package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"sort"
)

var fset *token.FileSet
var file *ast.File
var currentFunction *ast.FuncDecl
var hasErrors bool
var (
	optionIgnoreTests       bool
	optionMaxLineComplexity int
	optionNeverFail         bool
)
var commentGroupIndex int

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

		var err error
		fset = token.NewFileSet()
		file, err = parser.ParseFile(fset, currentFile, nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}

		commentGroupIndex = 0

		for _, decl := range file.Decls {
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

	// Body will be nil for non-Go (external) functions.
	if fn.Body == nil {
		return
	}

	for _, stmt := range fn.Body.List {
		checkLine(stmt)
	}
}

func consumeComment(line ast.Stmt) (comment string) {
	for commentGroupIndex < len(file.Comments) {
		commentGroup := file.Comments[commentGroupIndex]
		if commentGroup.Pos() < line.Pos() {
			comment += commentGroup.Text() + "\n"
			commentGroupIndex++
		} else {
			break
		}
	}

	return
}

func checkLine(line ast.Stmt) bool {
	complexity := LineComplexity(line)

	if complexity > optionMaxLineComplexity {
		printLine(complexity, line)
	}

	return complexity <= optionMaxLineComplexity
}

func LineComplexity(line ast.Stmt) int {
	// Check for ignore comment.
	comment := consumeComment(line)
	if strings.Contains(comment, "ghost:ignore") {
		return 0
	}

	defer func() {
		if r := recover(); r != nil {
			printLine(-1, line)
			panic(r)
		}
	}()

	switch n := line.(type) {
	case nil, *ast.LabeledStmt, *ast.SelectStmt:
		return 0

	case *ast.IncDecStmt, *ast.BranchStmt:
		return 1

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

	case *ast.BlockStmt:
		for _, l := range n.List {
			LineComplexity(l)
		}

		return 0

	case *ast.ForStmt:
		for _, l := range n.Body.List {
			LineComplexity(l)
		}

		// A "for" statement can contain multiple components. We have to
		// consider the complexity of each element and only return the maximum
		// complexity.
		//
		// The condComplexity does not have +1 added to it like you would expect
		// because it is expected that a binary expression containing the
		// iterator is included in the expression and this can not be further
		// simplified.
		initComplexity := LineComplexity(n.Init)
		condComplexity := exprComplexity(n.Cond)
		postComplexity := LineComplexity(n.Post)
		maxComplexity := maxInt(initComplexity, condComplexity, postComplexity)

		return maxComplexity

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

	case *ast.GoStmt:
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
			switch s := spec.(type) {
			case *ast.TypeSpec:
				// TypeSpec is an inline type. There is no complexity for these.

			case *ast.ValueSpec:
				total += exprsComplexity(s.Values)

			default:
				printLine(-1, line)
				panic(n)
			}
		}

		return total

	case *ast.CaseClause:
		for _, l := range n.Body {
			LineComplexity(l)
		}

		return listComplexity(n.List)

	case *ast.SendStmt:
		return exprComplexity(n.Value)

	default:
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
	case nil, *ast.BasicLit, *ast.Ident, *ast.ArrayType, *ast.MapType,
	*ast.ChanType, *ast.StructType, *ast.InterfaceType:
		return 0

	case *ast.SelectorExpr:
		// SelectorExpr is accessing a variable on a struct, like "foo.bar". A
		// SelectorExpr must be considered zero complexity for a few reasons:
		//
		// 1. To say "foo.bar" you must already have "foo" as a variable to
		// inspect. So "bar" is viewable by the debugger without any
		// intermediate step.
		//
		// 2. If we consider "foo.bar" to be of complexity 1 then function calls
		// like "foo(bar.baz, bar.qux)" will increase in complexity with each
		// argument. For the previous reason there is no need or benefit to
		// assigning each of the arguments into an intermediate variable to
		// decrease complexity.
		//
		// 3. The two previous rules also hold true when chaining multiple
		// selectors together like "foo.bar.baz".
		return 0

	case *ast.UnaryExpr, *ast.TypeAssertExpr:
		return 1

	case *ast.StarExpr:
		return exprComplexity(e.X)

	case *ast.CallExpr:
		return 1 + exprsComplexity(e.Args)

	case *ast.BinaryExpr:
		// BinaryExpr is a bit complicated to work out because there are some
		// edge cases that need to be considered.
		//
		// In the general case a binary expression will have a complexity of the
		// sum of it's left and right side. On top of that one complexity is
		// added if either side is a function call.
		//
		// Catching the function call on either side is important because the
		// returned value can always be assigned to an intermediate variable and
		// this is wise because the debugger cannot usually see this value.
		//
		// The minimum complexity for any binary expression is one. Even if both
		// sides have a complexity of zero. This prevents expressions that
		// contain many nested binary expressions to return an unreasonably low
		// complexity.
		//
		// The logical AND operator ("&&") is a special case when the left side
		// is an equality test against nil. Since it is very common to use && to
		// short-circuit a nil access like "c.a != nil && c.a.Foo()" we
		// effectively ignore the complexity on the left side. This is because
		// there is no reasonable way to reduce this kind of expression without
		// just making the code more verbose.

		left := exprComplexity(e.X)
		right := exprComplexity(e.Y)

		// Catch "&&" nil short-circuit.
		if x, ok := e.X.(*ast.BinaryExpr); ok && e.Op == token.LAND {
			if y, ok := x.Y.(*ast.Ident); ok && y.Name == "nil" {
				return right
			}
		}

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

	default:
		panic(e)
	}
}

func printLine(complexity int, line ast.Stmt) {
	hasErrors = true
	pos := fset.Position(line.Pos())

	functionName := ""
	if currentFunction != nil {
		functionName = currentFunction.Name.Name
	}

	fmt.Printf("%s:%d: complexity is %d (in %s)\n", pos.Filename, pos.Line,
		complexity, functionName)
}

func maxInt(numbers ...int) int {
	sort.Ints(numbers)

	return numbers[len(numbers)-1]
}
