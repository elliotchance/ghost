package main

import (
	"testing"
	"github.com/elliotchance/tf"
	"go/token"
	"github.com/stretchr/testify/assert"
	"fmt"
	"go/parser"
	"go/ast"
)

func TestLineComplexity(t *testing.T) {
	fn := func(line string) int {
		line = fmt.Sprintf("package p\nfunc a() { %s }", line)

		fset = token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", line, parser.ParseComments)
		assert.NoError(t, err)

		return LineComplexity(node.Decls[0].(*ast.FuncDecl).Body.List[0])
	}

	fnSwitch := func(line string) int {
		line = fmt.Sprintf("package p\nfunc a() { switch { %s } }", line)

		fset = token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", line, parser.ParseComments)
		assert.NoError(t, err)

		stmts := node.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.SwitchStmt).Body.List

		return LineComplexity(stmts[0])
	}

	LC := tf.NamedFunction(t, "Assignment", fn)
	LC(`hello := "Hello"`).Returns(0)
	LC(`hello := foo("bar")`).Returns(1)
	LC(`hello := foo("bar") + baz()`).Returns(3)
	LC(`hello := foo(baz())`).Returns(2)
	LC(`hello := "Hello" + name`).Returns(1)
	LC(`hello := "Hello" + name + again`).Returns(1)

	LC = tf.NamedFunction(t, "CallFunction", fn)
	LC(`foo("bar")`).Returns(1)
	LC(`foo(123)`).Returns(1)
	LC(`foo(1, 2, 3)`).Returns(1)
	LC(`foo(1 + 2, "foo")`).Returns(2)
	LC(`foo(1 + name, "foo")`).Returns(2)
	LC(`foo(1 + name, "foo" + name)`).Returns(3)
	LC(`foo(a.b, a.c, a.d, a.e)`).Returns(1)

	LC = tf.NamedFunction(t, "CompositeLiteral", fn)
	LC(`words := []string{}`).Returns(1)
	LC(`words := []string{hello, world}`).Returns(1)
	LC(`words := []string{hello, world, name}`).Returns(1)
	LC(`words := []string{hello + world, world}`).Returns(1)
	LC(`words := []string{hello + world + world}`).Returns(1)

	LC = tf.NamedFunction(t, "Binary", fn)
	LC(`3 + 2`).Returns(1)
	LC(`3 + 2 * 7`).Returns(1)
	LC(`3 + name * 7`).Returns(1)
	LC(`5.2 + 2`).Returns(1)
	LC(`"hello" + "world"`).Returns(1)
	LC(`foo + bar`).Returns(1)
	LC(`foo(bar) || baz`).Returns(2)
	LC(`true || foo(bar)`).Returns(2)
	LC(`foo(bar) || foo(bar)`).Returns(3)

	LC = tf.NamedFunction(t, "Return", fn)
	LC(`return`).Returns(0)
	LC(`return 123`).Returns(1)
	LC(`return 123 + 345`).Returns(1)
	LC(`return name`).Returns(1)
	LC(`return name + abc`).Returns(1)
	LC(`return name, abc`).Returns(1)
	LC(`return name, foo, bar`).Returns(1)
	LC(`return name + abc, abc`).Returns(1)
	LC(`return name + abc, abc + foo`).Returns(1)

	LC = tf.NamedFunction(t, "Unary", fn)
	LC(`!foo`).Returns(1)
	LC(`&foo`).Returns(1)
	LC(`!!foo`).Returns(1)

	LC = tf.NamedFunction(t, "If", fn)
	LC(`if false {}`).Returns(0)
	LC(`if foo {}`).Returns(0)
	LC(`if a || b {}`).Returns(1)

	LC = tf.NamedFunction(t, "Selector", fn)
	LC(`a.b`).Returns(0)
	LC(`a.b.c`).Returns(0)

	LC = tf.NamedFunction(t, "Declaration", fn)
	LC(`var a float64`).Returns(0)
	LC(`var a float64 = 3.2`).Returns(0)
	LC(`var a float64 = 3.2 + 7`).Returns(1)
	LC(`var a float64 = 3.2 + b`).Returns(1)
	LC(`var a float64 = foo(3.2 + b)`).Returns(2)

	LC = tf.NamedFunction(t, "Switch", fn)
	LC(`switch {}`).Returns(0)
	LC(`switch foo {}`).Returns(1)
	LC(`switch foo(bar) {}`).Returns(2)
	LC(`switch foo(bar) || baz {}`).Returns(3)

	LC = tf.NamedFunction(t, "Range", fn)
	LC(`for range foo {}`).Returns(0)
	LC(`for range foo.bar {}`).Returns(0)
	LC(`for range foo("bar") {}`).Returns(1)
	LC(`for range foo(bar()) {}`).Returns(2)

	LC = tf.NamedFunction(t, "Index", fn)
	LC(`a["b"]`).Returns(1)
	LC(`a[12 + 34]`).Returns(2)
	LC(`a[12 + foo]`).Returns(2)
	LC(`a["b"]["c"]`).Returns(1)

	LC = tf.NamedFunction(t, "KeyValueExpr", fn)
	LC(`map[int]int{1: 2}`).Returns(1)
	LC(`map[int]int{1: 2, 3: 4}`).Returns(1)
	LC(`map[int]int{1: foo(bar("bar")), 3: 4 + 2}`).Returns(2)

	LC = tf.NamedFunction(t, "Defer", fn)
	LC(`defer foo()`).Returns(0)
	LC(`defer func() {}()`).Returns(1)

	LC = tf.NamedFunction(t, "Paren", fn)
	LC(`a := (123)`).Returns(0)
	LC(`a := (((123)))`).Returns(0)
	LC(`a := (1 + 2)`).Returns(1)
	LC(`a := (foo + bar)`).Returns(1)
	LC(`a := (foo + bar + baz)`).Returns(1)

	LC = tf.NamedFunction(t, "FuncLit", fn)
	LC(`func () {}`).Returns(1)
	LC(`func (a, b int) {}`).Returns(1)
	LC(`func () { foo(bar(baz())); }`).Returns(1)

	LC = tf.NamedFunction(t, "For", fn)
	LC(`for {}`).Returns(0)
	LC(`for true {}`).Returns(1)
	LC(`for true && false {}`).Returns(2)

	LC = tf.NamedFunction(t, "TypeAssert", fn)
	LC(`a.(Foo)`).Returns(1)
	LC(`a.(Foo).b.(Bar)`).Returns(1)

	LC = tf.NamedFunction(t, "ArrayType", fn)
	LC(`make([]*NameNode, 123)`).Returns(1)
	LC(`make([]*NameNode, a + 23)`).Returns(2)

	LC = tf.NamedFunction(t, "Slice", fn)
	LC(`lastName[1 : 2]`).Returns(1)
	LC(`lastName[1 : foo]`).Returns(1)
	LC(`lastName[1 : foo + bar]`).Returns(2)
	LC(`lastName[baz() : foo + bar]`).Returns(3)
	LC(`lastName[baz() : foo + bar : 17]`).Returns(3)
	LC(`lastName[baz() : foo + bar : qux()]`).Returns(4)

	LC = tf.NamedFunction(t, "Star", fn)
	LC(`*foo`).Returns(0)
	LC(`*(foo + bar)`).Returns(1)

	LC = tf.NamedFunction(t, "TypeSwitch", fn)
	LC(`switch a.(type) {}`).Returns(1)

	LC = tf.NamedFunction(t, "Case", fnSwitch)
	LC(`case 123:`).Returns(1)
	LC(`case foo("bar"):`).Returns(1)
	LC(`case foo(bar()):`).Returns(2)
	LC(`case foo(bar()), baz(bar(bar())):`).Returns(4)

	LC = tf.NamedFunction(t, "Map", fn)
	LC(`make(map[string]int)`).Returns(1)

	LC = tf.NamedFunction(t, "Chan", fn)
	LC(`make(chan string)`).Returns(1)
}
