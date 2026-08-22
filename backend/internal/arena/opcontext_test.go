package arena

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpContextIsAlwaysBounded(t *testing.T) {
	ctx, cancel := opContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("opContext returned a context with no deadline")
	}
	left := time.Until(deadline)
	if left <= 0 || left > opTimeout+time.Second {
		t.Fatalf("deadline is %v away, want (0, %v]", left, opTimeout)
	}
	if ctx.Err() != nil {
		t.Fatalf("context already done: %v", ctx.Err())
	}

	cancel()
	if ctx.Err() == nil {
		t.Fatal("cancel() did not cancel the context")
	}
}

// TestNoBareBackgroundContextInArena keeps the hardening from rotting.
//
// Every Redis and Postgres call in this package is reached from a goroutine
// that outlives the request that started it -- a match loop, a queue timer, a
// WebSocket read pump -- so none of them can borrow a request context. They
// used to take context.Background() instead, which is unbounded: one wedged
// query and that goroutine is gone for the life of the process, holding its
// player's queue slot or match with it.
//
// opContext is the only sanctioned way to get one of those, so the rule this
// enforces is simply that context.Background() appears nowhere else here --
// opcontext.go is the single permitted site. The match loop shows the intended
// shape for long-lived work: it holds no context at all and wraps each step in
// its own bounded one, so a wedged query costs that match one step rather than
// the whole goroutine.
func TestNoBareBackgroundContextInArena(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var offenders []string

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Background" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "context" {
				return true
			}
			pos := fset.Position(call.Pos())
			if allowedBackground(name, pos.Line) {
				return true
			}
			offenders = append(offenders, filepath.Join(name)+":"+itoa(pos.Line))
			return true
		})
	}

	if len(offenders) > 0 {
		t.Fatalf("unbounded context.Background() outside the sanctioned sites: %v\n"+
			"use opContext() so a wedged query cannot strand the goroutine", offenders)
	}
}

// allowedBackground names the single file that may construct an unbounded
// parent: the one that immediately wraps it in a deadline.
func allowedBackground(file string, _ int) bool {
	return file == "opcontext.go"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
