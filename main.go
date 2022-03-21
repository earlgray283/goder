package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"log"
	"os"

	"golang.org/x/tools/imports"
)

const suffixLen int = 8

var (
	dst                 io.Writer = os.Stdout // 出力先
	overwrite           bool                  // 上書き
	onlyConvertGenerics bool                  // ジェネリクスの変換のみ
	onlyConvertExpkgs   bool                  // 外部パッケージの変換のみ
)

func init() {
	flag.BoolVar(&overwrite, "w", false, "write result to (source) file instead of stdout")
	flag.BoolVar(&onlyConvertGenerics, "g", false, "only convert generics")
	flag.BoolVar(&onlyConvertExpkgs, "e", false, "only convert external packages")
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println()
		fmt.Println("  goder [Options] <source_file>")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println()
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	filename := flag.Arg(0)
	srcFile, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer srcFile.Close()
	if overwrite {
		dst = srcFile
	}

	srcBytes := &bytes.Buffer{}
	if _, err := io.Copy(srcBytes, srcFile); err != nil {
		log.Fatal(err)
	}

	if onlyConvertExpkgs {
		convertedSrc, err := ConvertExternalPkgs(srcBytes.Bytes())
		if err != nil {
			log.Fatal(err)
		}
		if _, err := dst.Write(convertedSrc); err != nil {
			log.Fatal(err)
		}
		return
	}
	if onlyConvertGenerics {
		convertedSrc, err := ConvertGenerics(srcBytes.Bytes())
		if err != nil {
			log.Fatal(err)
		}
		if _, err := dst.Write(convertedSrc); err != nil {
			log.Fatal(err)
		}
		return
	}

	convertedSrc, err := convertAll(srcBytes.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if _, err := dst.Write(convertedSrc); err != nil {
		log.Fatal(err)
	}
}

func convertAll(src []byte) ([]byte, error) {
	noExpkgsSrc, err := ConvertExternalPkgs(src)
	if err != nil {
		log.Fatal(err)
	}
	return ConvertGenerics(noExpkgsSrc)
}

// create back-up file.
func createBak(bakPath string, data []byte) error {
	f, err := os.Create(bakPath + ".bak")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func formatAst(f any, fset *token.FileSet) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, f); err != nil {
		return nil, err
	}
	//fmt.Println(buf.String())
	return imports.Process("", buf.Bytes(), &imports.Options{})
}
