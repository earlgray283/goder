package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

const filename string = "examples/samber_lo.go"
const dstPath string = "/Users/earlgray/Workspace/Intern/Mercari/goder/tmp/single_samber_lo.go"

func main() {
	var externPkgHostSet = map[string]struct{}{
		"github.com": {},
		"golang.org": {},
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	// pkg -> local cache abs path
	externPkgMap := map[string]string{}
	for _, impt := range f.Imports {
		pkgName := strings.Trim(impt.Path.Value, "\"")
		hostname, rawPkgName := getHeadTail(strings.Split(pkgName, "/"))
		if _, ok := externPkgHostSet[hostname]; !ok {
			continue
		}

		// 外部パッケージのローカルキャッシュのパスを見つける
		pkgPath, err := getPkgAbsPath(build.Default.GOPATH, pkgName)
		if err != nil {
			log.Fatal(err)
		}
		externPkgMap[rawPkgName] = pkgPath
	}

	newSrc, err := deleteExPkgsAndFormat(f, fset, maps.Keys(externPkgMap)...)
	if err != nil {
		log.Fatal(err)
	}
	if err := createFileWithBytes(dstPath, newSrc); err != nil {
		log.Fatal(err)
	}
}

func createFileWithBytes(filename string, data []byte) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// delete external packages
func deleteExPkgsAndFormat(f *ast.File, fset *token.FileSet, externPkgs ...string) ([]byte, error) {
	externPkgSet := MakeSetFromSlice(externPkgs)
	fmt.Println(externPkgs)
	f2 := astutil.Apply(f, func(c *astutil.Cursor) bool {
		n, _ := c.Node().(*ast.SelectorExpr)
		if n == nil {
			return true
		}
		pkg, _ := n.X.(*ast.Ident)
		if pkg == nil {
			return true
		}
		fmt.Println("check", pkg.Name)
		if _, ok := externPkgSet[pkg.Name]; ok {
			c.Replace(n.Sel)
		}
		return true
	}, nil)
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, f2); err != nil {
		return nil, err
	}
	src, err := imports.Process("", buf.Bytes(), nil)
	if err != nil {
		return nil, err
	}
	return src, nil
}

// TODO: go.mod からバージョンをとれた方がいい(調べることが多そうなのでとりあえずはパターンマッチで・・)
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

type Set[T comparable] map[T]struct{}

func MakeSetFromSlice[T comparable](a []T) Set[T] {
	set := Set[T]{}
	for _, elem := range a {
		set[elem] = struct{}{}
	}
	return set
}

func getHeadTail[T any](a []T) (T, T) {
	return a[0], a[len(a)-1]
}
