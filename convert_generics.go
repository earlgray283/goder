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
	"golang.org/x/exp/maps"
)

func ConvertGenerics(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, err
	}
	config := &types.Config{Importer: importer.Default()}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
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

				typeParamTypeMap := detectTypeParamTypes(info, callExpr, genericFuncDecl)

				newFuncDecl := astcopy.FuncDecl(genericFuncDecl)   // funcDecl をコピーして新しい関数を作る
				newFuncDecl.Name.Name += makeRandString(suffixLen) // 名前が重複しないように suffix にランダム文字列を追加
				newFuncDecl.Type.TypeParams = nil                  // 型パラメーターを取る
				ast.Inspect(newFuncDecl, func(n ast.Node) bool {
					if ident, _ := n.(*ast.Ident); ident != nil {
						if basicType, ok := typeParamTypeMap[ident.Name]; ok {
							ident.Name = basicType
						}
					}
					return true
				})
				fmt.Println(typeParamTypeMap)
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

// T->string
// R->int みたいな
func detectTypeParamTypes(
	info *types.Info,
	callExpr *ast.CallExpr,
	funcDecl *ast.FuncDecl,
) map[string]string {
	typeParamTypeMap := map[string]string{}
	// int: int になりがちなのでそれらの要素を取り除く
	defer maps.DeleteFunc(typeParamTypeMap, func(k, v string) bool {
		return k == v
	})

	index := 0
	for _, arg := range funcDecl.Type.Params.List {
		for range arg.Names {
			funcDeclArg := arg.Type
			callExprArg := callExpr.Args[index]

			// 変数
			if ident, _ := callExprArg.(*ast.Ident); ident != nil {
				callExprType, funcDeclArgType := info.TypeOf(ident).String(), info.TypeOf(funcDeclArg).String()
				prefix := commonPrefix(callExprType, funcDeclArgType)
				typeParamTypeMap[funcDeclArgType[len(prefix):]] = callExprType[len(prefix):]
			}

			// 関数
			if funcLit, _ := callExprArg.(*ast.FuncLit); funcLit != nil {
				lambdaDeclType := funcDeclArg.(*ast.FuncType)
				// 引数から調べる
				for i2, arg2 := range lambdaDeclType.Params.List {
					lambdaDeclArg := arg2.Type.(*ast.Ident)
					funcLitArg := funcLit.Type.Params.List[i2].Type.(*ast.Ident)
					typeParamTypeMap[lambdaDeclArg.Name] = funcLitArg.Name
				}
				// 戻り値から調べる
				for i2, res2 := range lambdaDeclType.Results.List {
					lambdaDeclRes := res2.Type.(*ast.Ident)
					funcLitRes := funcLit.Type.Results.List[i2].Type.(*ast.Ident)
					typeParamTypeMap[lambdaDeclRes.Name] = funcLitRes.Name
				}
			}

			index++
		}
	}

	return typeParamTypeMap
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

func commonPrefix(s, t string) string {
	n := len(s)
	if len(t) < n {
		n = len(t)
	}
	common := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		if s[i] != t[i] {
			break
		}
		common = append(common, s[i])
	}
	return string(common)
}
