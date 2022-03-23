package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	examplesDirName = "_examples"
	outputDirName   = "_tmp"
	outputExt       = "_goder.go"
	goext           = ".go"
)

func TestConvertExternalPkgs(t *testing.T) {
	examplesDir, err := os.ReadDir(examplesDirName)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirEntry := range examplesDir {
		if !dirEntry.IsDir() {
			if strings.HasSuffix(dirEntry.Name(), ".go") {
				t.Log(dirEntry.Name())
				b, err := os.ReadFile(filepath.Join(examplesDirName, dirEntry.Name()))
				if err != nil {
					t.Fatal(err)
				}

				src, err := ConvertExternalPkgs(b)
				if err != nil {
					t.Log(err)
					t.Fail()
				}

				convertedFilename := dirEntry.Name()[:len(dirEntry.Name())-len(goext)] + outputExt
				f, err := os.Create(filepath.Join(outputDirName, convertedFilename))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				f.Write(src)
			}
		}
	}
}
