package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/earlgray283/astcopy"
	"github.com/earlgray283/kyopro-go"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/ast/astutil"
)

var externPkgHostSet = map[string]struct{}{
	"github.com": {},
	"golang.org": {},
}

func filterNoTest(f fs.FileInfo) bool {
	return !strings.HasSuffix(f.Name(), "_test.go")
}

func removeExTypeParams(funcDecl *ast.FuncDecl, externPkgSet kyopro.Set[string]) {
	if funcDecl.Type.TypeParams != nil {
		for _, field := range funcDecl.Type.TypeParams.List {
			if binaryExpr, _ := field.Type.(*ast.BinaryExpr); binaryExpr != nil {
				if selectorExpr, _ := binaryExpr.X.(*ast.SelectorExpr); selectorExpr != nil {
					if ident, _ := selectorExpr.X.(*ast.Ident); ident != nil {
						if !externPkgSet.ContainsKey(ident.Name) {
							continue
						}
					}
					binaryExpr.X = selectorExpr.Sel
				}
				if selectorExpr, _ := binaryExpr.Y.(*ast.SelectorExpr); selectorExpr != nil {
					if ident, _ := selectorExpr.X.(*ast.Ident); ident != nil {
						if !externPkgSet.ContainsKey(ident.Name) {
							continue
						}
					}
					binaryExpr.Y = selectorExpr.Sel
				}
			}
			if selectorExpr, _ := field.Type.(*ast.SelectorExpr); selectorExpr != nil {
				if ident, _ := selectorExpr.X.(*ast.Ident); ident != nil {
					if !externPkgSet.ContainsKey(ident.Name) {
						continue
					}
				}
				field.Type = selectorExpr.Sel
			}
		}
	}
}

func removeExPkgs(orgDecl ast.Decl, externPkgSet kyopro.Set[string]) ast.Decl {
	decl := astcopy.Decl(orgDecl)
	if funcDecl, _ := decl.(*ast.FuncDecl); funcDecl != nil {
		removeExTypeParams(funcDecl, externPkgSet)
	}
	astutil.Apply(decl, func(c *astutil.Cursor) bool {
		if selectorExpr, _ := c.Node().(*ast.SelectorExpr); selectorExpr != nil {
			if ident, _ := selectorExpr.X.(*ast.Ident); ident != nil {
				if externPkgSet.ContainsKey(ident.Name) {
					c.Replace(selectorExpr.Sel)
				}
			}
		}
		return true
	}, nil)
	return decl
}

func removeExPkgsAndAppend(
	target *ast.File,
	orgDecl ast.Decl,
	externPkgSet kyopro.Set[string],
) {
	decl := removeExPkgs(orgDecl, externPkgSet)
	target.Decls = append(target.Decls, decl)
}

func convertExternalPkgs(
	target *ast.File,
	pkg *ast.Package,
	visited kyopro.Set[string],
) error {
	for _, f := range pkg.Files {
		externPkgMap, err := makePkgCacheMap(f.Imports, externPkgHostSet)
		if err != nil {
			return err
		}
		externPkgSet := kyopro.MakeSetFromSlice(maps.Keys(externPkgMap))

		// 関数の中から外部パッケージの ident を削除して追加する
		for _, decl := range f.Decls {
			if genDecl, _ := decl.(*ast.GenDecl); genDecl != nil {
				if genDecl.Tok == token.IMPORT {
					continue
				}
			}
			removeExPkgsAndAppend(target, decl, externPkgSet)
		}

		// 外部パッケージのキャッシュから追加
		for _, cachePath := range externPkgMap {
			nextPkgs, err := parser.ParseDir(token.NewFileSet(), cachePath, filterNoTest, 0)
			if err != nil {
				return err
			}
			for _, nextPkg := range nextPkgs {
				if !visited.ContainsKey(nextPkg.Name) {
					visited.Insert(nextPkg.Name)
					if err := convertExternalPkgs(target, nextPkg, visited); err != nil {
						return err
					}
				}
			}
		}

	}
	return nil
}

func ConvertExternalPkgs(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, err
	}
	externPkgMap, err := makePkgCacheMap(f.Imports, externPkgHostSet)
	if err != nil {
		return nil, err
	}
	exterPkgSet := kyopro.MakeSetFromSlice(maps.Keys(externPkgMap))
	for i, decl := range f.Decls {
		if funcDecl, _ := decl.(*ast.FuncDecl); funcDecl != nil {
			f.Decls[i] = removeExPkgs(funcDecl, exterPkgSet).(*ast.FuncDecl)
		}
	}

	visited := kyopro.Set[string]{}
	for _, path := range externPkgMap {
		pkgs, err := parser.ParseDir(token.NewFileSet(), path, filterNoTest, 0)
		if err != nil {
			return nil, err
		}
		for _, pkg := range pkgs {
			visited.Insert(pkg.Name)
			if err := convertExternalPkgs(f, pkg, visited); err != nil {
				return nil, err
			}
		}
	}

	return formatAst(f, fset)
}

// pkg -> local cache abs path
func makePkgCacheMap(imports []*ast.ImportSpec, externPkgHostSet kyopro.Set[string]) (map[string]string, error) {
	externPkgMap := map[string]string{}

	for _, impt := range imports {
		pkgName := strings.Trim(impt.Path.Value, "\"")
		hostname, rawPkgName := getFirstLast(strings.Split(pkgName, "/"))
		if !externPkgHostSet.ContainsKey(hostname) {
			continue
		}

		// 外部パッケージのローカルキャッシュのパスを見つける
		pkgPath, err := getPkgAbsPath(build.Default.GOPATH, pkgName)
		if err != nil {
			return nil, err
		}

		// import で alias が貼ってあったらそれを使う
		if impt.Name != nil {
			externPkgMap[impt.Name.Name] = pkgPath
		} else {
			externPkgMap[rawPkgName] = pkgPath
		}
	}

	return externPkgMap, nil
}

// TODO: go.mod からバージョンをとれた方がいい(調べることが多そうなのでとりあえずはパターンマッチで・・)
// get absolute path which matches gopath/pkgName(pattern match)
func getPkgAbsPath(gopath, pkgName string) (string, error) {
	if !filepath.IsAbs(gopath) {
		return "", errors.New("gopath must be absolute path")
	}
	dirs := strings.Split(pkgName, "/")
	pkgPath := filepath.Join(gopath, "pkg", "mod")
	for i, pkgPattern := range dirs {
		entries, err := os.ReadDir(pkgPath)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if regexp.MustCompile(pkgPattern + "@.+").Match([]byte(entry.Name())) {
				pkgDirPath := filepath.Join(pkgPath, entry.Name())
				return filepath.Join(pkgDirPath, filepath.Join(dirs[i+1:]...)), nil
			}
		}
		pkgPath = pkgPath + "/" + pkgPattern
	}

	return "", fmt.Errorf(
		`Package "%v" not found in "%v". Please run following command.
  $ go get "%v"`,
		pkgName, filepath.Join(gopath, "pkg/mod"), pkgName,
	)
}

func getFirstLast[T any](a []T) (T, T) {
	return a[0], a[len(a)-1]
}
