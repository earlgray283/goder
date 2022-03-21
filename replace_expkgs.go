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

	"github.com/earlgray283/kyopro-go"
	"github.com/go-toolsmith/astcopy"
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

func appendDeclDepends(
	target *ast.File,
	orgDecl ast.Decl,
	externPkgMap map[string]string,
	visited kyopro.Set[string],
) error {
	var err error
	decl := astcopy.Decl(orgDecl)
	astutil.Apply(decl, func(c *astutil.Cursor) bool {
		if selectorExpr, _ := c.Node().(*ast.SelectorExpr); selectorExpr != nil {
			if ident, _ := selectorExpr.X.(*ast.Ident); ident != nil {
				if cachePath, ok := externPkgMap[ident.Name]; ok {
					nextPkgs, err2 := parser.ParseDir(token.NewFileSet(), cachePath, filterNoTest, 0)
					if err2 != nil {
						err = err2
						return false
					}
					for _, nextPkg := range nextPkgs {
						if !visited.ContainsKey(nextPkg.Name) {
							visited.Insert(nextPkg.Name)
							if err2 := convertExternalPkgs(target, nextPkg, visited); err2 != nil {
								err = err2
								return false
							}
						}
					}
					c.Replace(selectorExpr.Sel)
				}
			}
		}
		return true
	}, nil)
	if err != nil {
		return err
	}
	target.Decls = append(target.Decls, decl)
	return nil
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
		for _, decl := range f.Decls {
			// 関数
			if funcDecl, _ := decl.(*ast.FuncDecl); funcDecl != nil {
				if err := appendDeclDepends(target, funcDecl, externPkgMap, visited); err != nil {
					return err
				}
			}
			// 構造体
			if genDecl, _ := decl.(*ast.GenDecl); genDecl != nil {
				if genDecl.Tok == token.TYPE {
					if err := appendDeclDepends(target, genDecl, externPkgMap, visited); err != nil {
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

	f2 := deleteExternalPkgs(f, fset, maps.Keys(externPkgMap)...)

	visited := kyopro.Set[string]{}
	for _, path := range externPkgMap {
		pkgs, err := parser.ParseDir(token.NewFileSet(), path, filterNoTest, 0)
		if err != nil {
			return nil, err
		}
		for _, pkg := range pkgs {
			visited[pkg.Name] = struct{}{}
			if err := convertExternalPkgs(f2, pkg, visited); err != nil {
				return nil, err
			}
		}
	}

	return formatAst(f2, fset)
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

// delete external packages in src
// return src, map[pkgName][]funcNames, error
func deleteExternalPkgs(f *ast.File, fset *token.FileSet, externPkgs ...string) *ast.File {
	externPkgSet := kyopro.MakeSetFromSlice(externPkgs)
	f2 := astutil.Apply(f, func(c *astutil.Cursor) bool {
		n, _ := c.Node().(*ast.SelectorExpr)
		if n == nil {
			return true
		}
		pkg, _ := n.X.(*ast.Ident)
		if pkg == nil {
			return true
		}
		if externPkgSet.ContainsKey(pkg.Name) {
			c.Replace(n.Sel)
		}
		return true
	}, nil)
	return f2.(*ast.File)
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
