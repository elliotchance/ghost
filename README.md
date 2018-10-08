👻 ghost
========

[![Build Status](https://travis-ci.org/elliotchance/ghost.svg?branch=master)](https://travis-ci.org/elliotchance/ghost)

`ghost` is a command-line tool for locating overly complex lines of code in Go.

It is designed with the intention that code should strive to be written in a
linear, rather than nested way. This makes code easier to understand, highlights
duplicate logic and ultimately leads to less bugs.


Installation
------------

```bash
go get -u github.com/elliotchance/ghost
```


Usage
-----

Pass one or multiple Go files:

```bash
ghost file1.go file2.go
```

### CLI Options

- `-ignore-tests` - Ignore test files.
- `-max-line-complexity` - The maximum allowed line complexity. (default 5)
- `-never-fail` - Always exit with 0.


Example
-------

The output of ghost (with default options) describes that line 50 is too
complex:

```
jaro.go:50: complexity is 8 (in JaroWinkler)
```

The line is:

```go
prefixSize = int(math.Min(float64(len(a)), math.Min(float64(prefixSize), float64(len(b)))))
```

There is nothing logically incorrect with that line, but it is long, difficult
to understand and can be tricky to inspect with a debugger.

There are lots of different ways the above code can be rewritten. For me, once I
understand what it's really doing I can create the function:

```go
func minInt(values ...int) int {
	sort.Ints(values)

	return values[0]
}
```

Now it can be simply written as:

```go
prefixSize = minInt(prefixSize, len(a), len(b))
```
