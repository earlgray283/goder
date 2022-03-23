package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	examplesDirName      = "_examples"
	outputDirName        = "_tmp"
	convertedExpkgsExt   = "_no_expkgs_goder.go"
	convertedGenericsExt = "_no_generics_goder.go"
	goext                = ".go"
)

func TestConvertGenerics(t *testing.T) {
	examplesDir, err := os.ReadDir(examplesDirName)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirEntry := range examplesDir {
		if !dirEntry.IsDir() {
			if strings.HasSuffix(dirEntry.Name(), ".go") {
				examplesFilepath := filepath.Join(examplesDirName, dirEntry.Name())
				t.Log(examplesFilepath)
				b, err := os.ReadFile(examplesFilepath)
				if err != nil {
					t.Fatal(err)
				}

				src, err := ConvertExternalPkgs(b)
				if err != nil {
					t.Log(err)
					t.Fail()
				}
				src2, err := ConvertGenerics(src)
				if err != nil {
					t.Log(err)
					t.Fail()
				}

				convertedFilepath := filepath.Join(outputDirName, dirEntry.Name()[:len(dirEntry.Name())-len(goext)]+convertedGenericsExt)
				f, err := os.Create(convertedFilepath)
				if err != nil {
					t.Log(err)
					t.Fail()
				}
				f.Write(src2)
				f.Close()

				if err := checkCompile(convertedFilepath); err != nil {
					t.Log(err)
					t.Fail()
				}
			}
		}
	}
}

func TestConvertExternalPkgs(t *testing.T) {
	examplesDir, err := os.ReadDir(examplesDirName)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirEntry := range examplesDir {
		if !dirEntry.IsDir() {
			if strings.HasSuffix(dirEntry.Name(), ".go") {
				examplesFilepath := filepath.Join(examplesDirName, dirEntry.Name())
				t.Log(examplesFilepath)
				b, err := os.ReadFile(examplesFilepath)
				if err != nil {
					t.Fatal(err)
				}

				src, err := ConvertExternalPkgs(b)
				if err != nil {
					t.Log(err)
					t.Fail()
				}

				convertedFilepath := filepath.Join(outputDirName, dirEntry.Name()[:len(dirEntry.Name())-len(goext)]+convertedExpkgsExt)
				f, err := os.Create(convertedFilepath)
				if err != nil {
					t.Log(err)
					t.Fail()
				}
				f.Write(src)
				f.Close()

				if err := checkCompile(convertedFilepath); err != nil {
					t.Log(err)
					t.Fail()
				}
			}
		}
	}
}

func checkCompile(p string) error {
	cmd := exec.Command("go", "run", p)
	cmd.Stdout = io.Discard
	return cmd.Run()
}
