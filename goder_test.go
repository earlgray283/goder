package main

import (
	"os"
	"testing"
)

func TestConvertExternalPkgs(t *testing.T) {
	b, err := os.ReadFile("examples/ac-library-go_segtree.go")
	if err != nil {
		t.Fatal(err)
	}
	src, err := ConvertExternalPkgs(b)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create("tmp/ac-library-go_segtree_goder.go")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write(src)
}
