package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samber/lo"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/ast/astutil"
)

var externPkgHostSet = map[string]struct{}{
	"github.com": {},
	"golang.org": {},
}

func ReplaceExternalPkgs(filename string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	externPkgMap, err := makePkgCacheMap(f.Imports, externPkgHostSet)
	if err != nil {
		return nil, err
	}
	exPkgs, exPkgCachePathes := maps.Keys(externPkgMap), maps.Values(externPkgMap)

	/* 多分ループになる */
	noExPkgSrc, pkgFuncsMap, err := deleteExPkgsAndFormat(f, fset, exPkgs...)
	if err != nil {
		return nil, err
	}

	f, err = parser.ParseFile(fset, "", noExPkgSrc, 0)
	if err != nil {
		return nil, err
	}

	if err := appendExPkgFunc(f, exPkgCachePathes, pkgFuncsMap); err != nil {
		return nil, err
	}
	/* ... */

	return formatAst(f, fset)
}

// pkg -> local cache abs path
func makePkgCacheMap(imports []*ast.ImportSpec, externPkgHostSet Set[string]) (map[string]string, error) {
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
		externPkgMap[rawPkgName] = pkgPath
	}

	return externPkgMap, nil
}

// delete external packages in src
// return src, map[pkgName][]funcNames, error
func deleteExPkgsAndFormat(f *ast.File, fset *token.FileSet, externPkgs ...string) ([]byte, map[string][]string, error) {
	pkgFuncsMap := map[string][]string{}
	externPkgSet := MakeSetFromSlice(externPkgs)
	log.Println(externPkgs)
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
	pkgDir, pkgPattern := filepath.Split(pkgName)
	dirPath := filepath.Join(gopath, "pkg", "mod", pkgDir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if regexp.MustCompile(pkgPattern + "@.*").Match([]byte(entry.Name())) {
			return filepath.Join(dirPath, entry.Name()), nil
		}
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
			for fname, f := range pkg.Files {
				for _, decl := range f.Decls {
					funcDecl, _ := decl.(*ast.FuncDecl)
					if funcDecl == nil {
						continue
					}
					if !lo.Contains(pkgFuncsMap[pkgName], funcDecl.Name.Name) {
						continue
					}

					dst.Decls = append(dst.Decls, funcDecl)
					log.Printf("fname: %v -> func: %v\n", fname, funcDecl.Name)
				}
			}
		}
	}

	return nil
}
