package main

import (
	"bytes"
	"go/format"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/imports"
)

const (
	filename  string = "examples/samber_lo.go"
	dstPath   string = "/Users/earlgray/Workspace/Intern/Mercari/goder/tmp/single_samber_lo.go"
	suffixLen int    = 8
)

func main() {
	srcBytes, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	src, err := ReplaceExternalPkgs(srcBytes)
	if err != nil {
		log.Fatal(err)
	}

	src2, err := ConvertGenerics(src)
	if err != nil {
		log.Fatal(err)
	}

	if err := createFileWithBytes(dstPath, src2); err != nil {
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

func formatAst(f any, fset *token.FileSet) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, f); err != nil {
		return nil, err
	}
	return imports.Process("", buf.Bytes(), nil)
}
