package engine

import "testing"

func TestDOMCacheReusesOnlyTheCurrentHTMLDocument(t *testing.T) {
	cache := newDOMCache()
	firstHTML := `<article><h1>first</h1></article>`
	secondHTML := `<article><h1>second</h1></article>`

	first, err := cache.document(firstHTML)
	if err != nil {
		t.Fatalf("parse first document: %v", err)
	}
	again, err := cache.document(firstHTML)
	if err != nil {
		t.Fatalf("read cached document: %v", err)
	}
	if first != again {
		t.Fatal("same HTML in one task should reuse its parsed document")
	}

	second, err := cache.document(secondHTML)
	if err != nil {
		t.Fatalf("parse second document: %v", err)
	}
	if second == first || second.Find("h1").Text() != "second" {
		t.Fatal("different HTML must not reuse the previous parsed document")
	}
}
