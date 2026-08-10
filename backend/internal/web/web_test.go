package web

import "testing"

func TestDistFS_ContainsIndexHTML(t *testing.T) {
	fsys, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v", err)
	}
	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatalf("open index.html: %v", err)
	}
	defer func() { _ = f.Close() }()
}
