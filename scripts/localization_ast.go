//go:build ignore

// Offline source inspection helper. Uses only the Go standard library and does
// not type-check, initialize, or execute any application package.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type literal struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Context string `json:"context"`
	Value   string `json:"value"`
}

type inventory struct {
	Files     int               `json:"files"`
	Literals  []literal         `json:"literals"`
	Constants map[string]string `json:"constants"`
}

func hasEastAsian(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Bopomofo, r) {
			return true
		}
	}
	return false
}

func constString(e ast.Expr, declarations map[string]ast.Expr, seen map[string]bool) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			v, err := strconv.Unquote(e.Value)
			return v, err == nil
		}
	case *ast.ParenExpr:
		return constString(e.X, declarations, seen)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			a, aOK := constString(e.X, declarations, seen)
			b, bOK := constString(e.Y, declarations, seen)
			return a + b, aOK && bOK
		}
	case *ast.Ident:
		if !seen[e.Name] {
			if value, ok := declarations[e.Name]; ok {
				seen[e.Name] = true
				defer delete(seen, e.Name)
				return constString(value, declarations, seen)
			}
		}
	}
	return "", false
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	fset := token.NewFileSet()
	out := inventory{Constants: map[string]string{}, Literals: []literal{}}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "work", "node_modules", "vendor", ".next", "out":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		out.Files++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		declarations := map[string]ast.Expr{}
		for _, d := range file.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
				for _, spec := range g.Specs {
					v := spec.(*ast.ValueSpec)
					for i, name := range v.Names {
						if i < len(v.Values) {
							declarations[name.Name] = v.Values[i]
						}
					}
				}
			}
		}
		for name, value := range declarations {
			if value, ok := constString(value, declarations, map[string]bool{}); ok {
				out.Constants[rel+"::"+name] = value
			}
		}
		for _, declaration := range file.Decls {
			context := "declaration"
			if fn, ok := declaration.(*ast.FuncDecl); ok {
				context = fn.Name.Name
			}
			ast.Inspect(declaration, func(node ast.Node) bool {
				v, ok := node.(*ast.BasicLit)
				if !ok || v.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(v.Value)
				if err != nil {
					return true
				} // parser already rejects malformed literals
				if hasEastAsian(value) {
					out.Literals = append(out.Literals, literal{rel, fset.Position(v.Pos()).Line, context, value})
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
