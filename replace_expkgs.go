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

	"github.com/go-toolsmith/astcopy"
	"github.com/samber/lo"
	"golang.org/x/tools/go/ast/astutil"
)

var externPkgHostSet = map[string]struct{}{
	"github.com": {},
	"golang.org": {},
}

func filterNoTest(f fs.FileInfo) bool {
	return !filepath.HasPrefix(f.Name(), "_test.go")
}

func appendDeclDepends(
	target *ast.File,
	orgDecl ast.Decl,
	externPkgMap map[string]string,
	visited set[string],
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
						if _, ok := visited[nextPkg.Name]; !ok {
							visited[nextPkg.Name] = struct{}{}
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
		return false
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
	visited set[string],
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
	visited := set[string]{}
	for _, path := range externPkgMap {
		pkgs, err := parser.ParseDir(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, err
		}
		for _, pkg := range pkgs {
			visited[pkg.Name] = struct{}{}
			if err := convertExternalPkgs(f, pkg, visited); err != nil {
				return nil, err
			}
		}
	}

	return formatAst(f, fset)
}

// pkg -> local cache abs path
func makePkgCacheMap(imports []*ast.ImportSpec, externPkgHostSet set[string]) (map[string]string, error) {
	externPkgMap := map[string]string{}

	for _, impt := range imports {
		pkgName := strings.Trim(impt.Path.Value, "\"")
		hostname, rawPkgName := getFirstLast(strings.Split(pkgName, "/"))
		if _, ok := externPkgHostSet[hostname]; !ok {
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
func deleteExPkgsAndFormat(f *ast.File, fset *token.FileSet, externPkgs ...string) ([]byte, map[string][]string, error) {
	pkgFuncsMap := map[string][]string{}
	externPkgSet := makeSetFromSlice(externPkgs)
	f2 := astutil.Apply(f, func(c *astutil.Cursor) bool {
		n, _ := c.Node().(*ast.SelectorExpr)
		if n == nil {
			return true
		}
		pkg, _ := n.X.(*ast.Ident)
		if pkg == nil {
			return true
		}
		if _, ok := externPkgSet[pkg.Name]; ok {
			pkgFuncsMap[pkg.Name] = append(pkgFuncsMap[pkg.Name], n.Sel.Name)
			c.Replace(n.Sel)
		}
		return true
	}, nil)

	src, err := formatAst(f2, fset)

	return src, pkgFuncsMap, err
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

func appendExPkgFunc(dst *ast.File, exPkgCachePathes []string, pkgFuncsMap map[string][]string) error {
	for _, exPkgPath := range exPkgCachePathes {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, exPkgPath, func(fi fs.FileInfo) bool { return true }, 0)
		if err != nil {
			return err
		}
		for pkgName, pkg := range pkgs {
			for _, f := range pkg.Files {
				for _, decl := range f.Decls {
					funcDecl, _ := decl.(*ast.FuncDecl)
					if funcDecl == nil {
						continue
					}
					if !lo.Contains(pkgFuncsMap[pkgName], funcDecl.Name.Name) {
						continue
					}

					dst.Decls = append(dst.Decls, funcDecl)
					//log.Printf("fname: %v -> func: %v\n", fname, funcDecl.Name)
				}
			}
		}
	}

	return nil
}

func getFirstLast[T any](a []T) (T, T) {
	return a[0], a[len(a)-1]
}

type set[K comparable] map[K]struct{}

func makeSetFromSlice[K comparable](a []K) set[K] {
	set := set[K]{}
	for _, elem := range a {
		set[elem] = struct{}{}
	}
	return set
}
