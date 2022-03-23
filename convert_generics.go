package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"math/rand"

	"github.com/earlgray283/astcopy"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/ast/astutil"
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
	genericsStructs := findGenericsStructs(f.Decls)
	newDecls := make([]ast.Decl, 0)
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

				newFuncName := genericFuncDecl.Name.Name + makeRandString(suffixLen)
				ident.Name = newFuncName
				newFuncDecl := astcopy.FuncDecl(genericFuncDecl)
				newFuncDecl.Name.Name = newFuncName
				newFuncDecl.Type.TypeParams = nil

				// 型パラメータを基本型に変換
				astutil.Apply(newFuncDecl, func(c *astutil.Cursor) bool {
					if ident, _ := c.Node().(*ast.Ident); ident != nil {
						if basicType, ok := typeParamTypeMap[ident.Name]; ok {
							ident.Name = basicType
						}
					}
					if indexExpr, _ := c.Node().(*ast.IndexExpr); indexExpr != nil {
						c.Replace(indexExpr.X)
					}
					return true
				}, nil)

				if newFuncDecl.Recv != nil {
					for _, field := range newFuncDecl.Recv.List {
						starExpr := field.Type.(*ast.StarExpr)
						if indexExpr, _ := starExpr.X.(*ast.IndexExpr); indexExpr != nil {
							if basicType, ok := typeParamTypeMap[indexExpr.Index.(*ast.Ident).Name]; ok {
								ident.Name = basicType
							}
						}
					}
				}

				newDecls = append(newDecls, newFuncDecl)

				// 引数や戻り値の構造体の型パラメータを基本型に変換
				for _, res := range newFuncDecl.Type.Results.List {
					ast.Inspect(res.Type, func(n ast.Node) bool {
						if ident, _ := n.(*ast.Ident); ident != nil {
							if ident.Obj != nil {
								orgTypeSpec, _ := ident.Obj.Decl.(*ast.TypeSpec)
								if orgTypeSpec == nil {
									return true
								}
								if orgGenDecl, ok := genericsStructs[orgTypeSpec.Name.Name]; ok {
									genDecl := astcopy.GenDecl(orgGenDecl)
									typeSpec := astcopy.TypeSpec(orgTypeSpec)
									typeSpec.Name.Name += makeRandString(8)
									typeSpec.TypeParams = nil
									if structType, _ := typeSpec.Type.(*ast.StructType); structType != nil {
										for _, tp := range structType.Fields.List {
											ast.Inspect(tp, func(n2 ast.Node) bool {
												if funcType, _ := n2.(*ast.FuncType); funcType != nil {
													for _, field := range funcType.Params.List {
														if ident2, _ := field.Type.(*ast.Ident); ident2 != nil {
															if basicType, ok := typeParamTypeMap[ident2.Name]; ok {
																ident2.Name = basicType
															}
														}
													}
												}
												if ident2, _ := n2.(*ast.Ident); ident2 != nil {
													if basicType, ok := typeParamTypeMap[ident2.Name]; ok {
														ident2.Name = basicType
													}
												}
												return true
											})
										}
									}
									genDecl.Specs[0] = typeSpec
									newDecls = append(newDecls, genDecl)
									return true
								}
								return true
							}
						}
						return true
					})
				}

				return true
			})
		}
	}
	f.Decls = append(f.Decls, newDecls...)

	// ジェネリックな関数や構造体の削除
	noGenericDecls := make([]ast.Decl, 0)
	for _, decl := range f.Decls {
		if funcDecl, _ := decl.(*ast.FuncDecl); funcDecl != nil {
			if funcDecl.Type.TypeParams != nil {
				continue
			}
		}
		// if genDecl, _ := decl.(*ast.GenDecl); genDecl != nil {
		// 	for _, spec := range genDecl.Specs {
		// 		typeSpec, _ := spec.(*ast.TypeSpec)
		// 		if typeSpec == nil {
		// 			continue
		// 		}
		// 		structType, _ := typeSpec.Type.(*ast.StructType)
		// 		if structType == nil {
		// 			continue
		// 		}
		// 		for _, field := range structType.Fields.List {
		// 			for _, ident := range field.Names {
		// 				if
		// 			}
		// 		}

		// 		typeSpec.TypeParams = nil
		// 	}
		// }
		noGenericDecls = append(noGenericDecls, decl)
	}
	f.Decls = noGenericDecls

	return formatAst(f, fset)
}

func detectTypeParamTypes(
	info *types.Info,
	callExpr *ast.CallExpr,
	funcDecl *ast.FuncDecl,
) map[string]string {
	typeParamTypeMap := map[string]string{}
	index := 0
	for _, arg := range funcDecl.Type.Params.List {
		for range arg.Names {
			funcDeclArg := arg.Type
			callExprArg := callExpr.Args[index]

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
					// T
					if funcLitResIdent, _ := funcLit.Type.Results.List[i2].Type.(*ast.Ident); funcLitResIdent != nil {
						typeParamTypeMap[lambdaDeclRes.Name] = funcLitResIdent.Name
					}
					// *T
					if funcLitResStarExpr, _ := funcLit.Type.Results.List[i2].Type.(*ast.StarExpr); funcLitResStarExpr != nil {
						typeParamTypeMap[lambdaDeclRes.Name] = "*" + funcLitResStarExpr.X.(*ast.Ident).Name
					}
				}
				index++
				continue
			}

			// 変数や expr
			typ := info.TypeOf(callExprArg).String()
			if ident, _ := funcDeclArg.(*ast.Ident); ident != nil {
				typeParamTypeMap[ident.Name] = typ
			}

			index++
		}
	}

	maps.DeleteFunc(typeParamTypeMap, func(k, v string) bool {
		return k == v
	})

	return typeParamTypeMap
}

// find functions which use generics
func findGenericsFuncs(decls []ast.Decl) map[string]*ast.FuncDecl {
	genericsFuncs := map[string]*ast.FuncDecl{}
	for _, decl := range decls {
		if funcDecl, _ := decl.(*ast.FuncDecl); funcDecl != nil {
			if funcDecl.Type.TypeParams != nil {
				genericsFuncs[funcDecl.Name.Name] = funcDecl
			}
			if funcDecl.Recv != nil {
				for _, field := range funcDecl.Recv.List {
					flg := false
					ast.Inspect(field, func(n ast.Node) bool {
						if indexExpr, _ := n.(*ast.IndexExpr); indexExpr != nil {
							flg = true
							return false
						}
						return true
					})
					if flg {
						genericsFuncs[funcDecl.Name.Name] = funcDecl
					}
				}
			}
		}
	}
	return genericsFuncs
}

// find structs which use generics
func findGenericsStructs(decls []ast.Decl) map[string]*ast.GenDecl {
	genericsStructs := map[string]*ast.GenDecl{}
	for _, decl := range decls {
		if genDecl, _ := decl.(*ast.GenDecl); genDecl != nil {
			for _, spec := range genDecl.Specs {
				if typeSpec, _ := spec.(*ast.TypeSpec); typeSpec != nil {
					if typeSpec.TypeParams != nil {
						genericsStructs[typeSpec.Name.Name] = genDecl
					}
				}
			}
		}
	}
	return genericsStructs
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
