package main

import (
	"bytes"
	"go/format"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/imports"
)

const filename string = "examples/samber_lo.go"
const dstPath string = "/Users/earlgray/Workspace/Intern/Mercari/goder/tmp/single_samber_lo.go"

func main() {
	src, err := ReplaceExternalPkgs(filename)
	if err != nil {
		log.Fatal(err)
	}

	if err := createFileWithBytes(dstPath, src); err != nil {
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
