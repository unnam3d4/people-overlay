package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRepairsSameSizeCorruption(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, name := range resourceFiles() {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write(bytes.Repeat([]byte{42}, 256))
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	payload := append([]byte("test executable prefix"), b.Bytes()...)
	payload = append(payload, bundleMagic...)
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(b.Len()))
	payload = append(payload, size[:]...)
	root := t.TempDir()
	exe := filepath.Join(root, "test.exe")
	dest := filepath.Join(root, "assets")
	if e := os.WriteFile(exe, payload, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := extractBundle(exe, dest); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dest, resourceFiles()[0])
	if e := os.WriteFile(file, bytes.Repeat([]byte{99}, 256), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := extractBundle(exe, dest); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(got, bytes.Repeat([]byte{42}, 256)) {
		t.Fatal("same-sized corrupt cache shadows bundled model")
	}
}
