package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"math/rand"

	"github.com/go-toolsmith/astcopy"
)

func ConvertGenerics(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, err
	}
	config := &types.Config{Importer: importer.Default()}
	info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
	if _, err := config.Check("main", fset, []*ast.File{f}, info); err != nil {
		return nil, err
	}

	genericsFuncs := findGenericsFuncs(f.Decls)
	newFuncDecls := make([]ast.Decl, 0)
	for _, decl := range f.Decls {
		funcDecl, _ := decl.(*ast.FuncDecl)
		if funcDecl == nil {
			continue
		}
		for _, stmt := range funcDecl.Body.List {
			ast.Inspect(stmt, func(n ast.Node) bool {
				callExpr, _ := n.(*ast.CallExpr)
				if callExpr == nil {
					return true
				}
				ident, _ := callExpr.Fun.(*ast.Ident)
				if ident == nil {
					return true
				}
				genericFuncDecl, ok := genericsFuncs[ident.Name]
				if !ok {
					return true
				}

				newFuncDecl, err2 := removeFuncTypeParams(info, callExpr, genericFuncDecl)
				if err2 != nil {
					err = err2
					return true
				}
				newFuncDecls = append(newFuncDecls, newFuncDecl)

				return true
			})
			if err != nil {
				return nil, err
			}
		}
	}
	f.Decls = append(f.Decls, newFuncDecls...)

	return formatAst(f, fset)
}

func removeFuncTypeParams(
	info *types.Info,
	callExpr *ast.CallExpr,
	funcDecl *ast.FuncDecl,
) (*ast.FuncDecl, error) {
	typeParamTypeMap := map[string]types.Type{}

	newFuncDecl := astcopy.FuncDecl(funcDecl)
	newFuncDecl.Name.Name += makeRandString(suffixLen)
	newFuncDecl.Type.TypeParams = nil
	//paramList := newFuncDecl.Type.Params.List
	for _, arg := range callExpr.Args {
		// 変数
		if ident, _ := arg.(*ast.Ident); ident != nil {
			o := info.Uses[ident]
			if o == nil {
				continue
			}
			typeParamTypeMap[ident.Name] = o.Type()
			fmt.Println(o.Type())

		}
		// 関数呼び出し
		if callExpr, _ := arg.(*ast.CallExpr); callExpr != nil {
			fmt.Println("unimplement!")
		}
	}
	return newFuncDecl, nil
}

// find functions which use generics
func findGenericsFuncs(decls []ast.Decl) map[string]*ast.FuncDecl {
	genericsFuncs := map[string]*ast.FuncDecl{}
	for _, decl := range decls {
		funcDecl, _ := decl.(*ast.FuncDecl)
		if funcDecl == nil {
			continue
		}
		if funcDecl.Type.TypeParams == nil {
			continue
		}
		genericsFuncs[funcDecl.Name.Name] = funcDecl
	}
	return genericsFuncs
}

// note: start of a sentence is big letter
func makeRandString(n int) string {
	const bigLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const smallLetters = "abcdefghijklmnopqrstuvwxyz"
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	b[0] = bigLetters[rand.Intn(len(bigLetters))]
	for i := 1; i < len(b); i++ {
		b[i] = smallLetters[rand.Intn(len(smallLetters))]
	}
	return string(b)
}
