package book

import "testing"

func TestNewResourceSet_SliceCopy(t *testing.T) {
	r1 := &Resource{Id: "a", ResolvedHref: "a.html"}
	r2 := &Resource{Id: "b", ResolvedHref: "b.html"}
	all := []*Resource{r1, r2}
	ro := []ReadingOrderItem{{Resource: r1, Linear: true}}
	rs := NewResourceSet(all, ro, nil)
	// Mutate caller slices - should not affect ResourceSet
	all[0] = nil
	_ = append(all, &Resource{Id: "c", ResolvedHref: "c.html"})
	ro[0].Linear = false

	if len(rs.All()) != 2 {
		t.Fatalf("All() len = %d, want 2 (defensive copy)", len(rs.All()))
	}
	if len(rs.ReadingOrder()) != 1 || !rs.ReadingOrder()[0].Linear {
		t.Fatalf("ReadingOrder defensive copy failed: %+v", rs.ReadingOrder())
	}
	if _, ok := rs.GetByID("c"); ok {
		t.Fatalf("GetByID found mutated entry")
	}
	if all[0] != nil {
		t.Fatalf("caller all[0] should be nil after mutation")
	}
}

func TestNewResourceSet_NilFiltered(t *testing.T) {
	all := []*Resource{nil, {Id: "a", ResolvedHref: "a.html"}, nil}
	ro := []ReadingOrderItem{
		{Resource: nil, Linear: true},
		{Resource: &Resource{Id: "a", ResolvedHref: "a.html"}, Linear: true},
		{Resource: nil, Linear: false},
	}
	rs := NewResourceSet(all, ro, nil)
	if len(rs.All()) != 1 {
		t.Fatalf("All() should filter nil, got %d", len(rs.All()))
	}
	if len(rs.ReadingOrder()) != 1 {
		t.Fatalf("ReadingOrder() should filter nil Resource, got %d", len(rs.ReadingOrder()))
	}
}

func TestGetByHref_FragmentQueryStripped(t *testing.T) {
	r := &Resource{Id: "cover", ResolvedHref: "OEBPS/images/cover.jpg"}
	rs := NewResourceSet([]*Resource{r}, nil, r)
	tests := []string{
		"OEBPS/images/cover.jpg",
		"OEBPS/images/cover.jpg#frag",
		"OEBPS/images/cover.jpg?query=1",
		"OEBPS/images/cover.jpg?query=1#frag",
		"OEBPS/images/cover.jpg#frag?query",
	}
	for _, href := range tests {
		if _, ok := rs.GetByHref(href); !ok {
			t.Errorf("GetByHref(%q) should find cover", href)
		}
	}
}

func TestGetByHref_OnlyResolvedHref(t *testing.T) {
	r := &Resource{Id: "ch1", Href: "ch1.html", ResolvedHref: "OEBPS/ch1.html"}
	rs := NewResourceSet([]*Resource{r}, nil, nil)
	if _, ok := rs.GetByHref("ch1.html"); ok {
		t.Errorf("GetByHref with original relative Href should not find; only ResolvedHref indexed")
	}
	if _, ok := rs.GetByHref("OEBPS/ch1.html"); !ok {
		t.Errorf("GetByHref with ResolvedHref should find")
	}
}

func TestGetByHref_CanonicalPathClean(t *testing.T) {
	r := &Resource{Id: "a", ResolvedHref: "OEBPS/images/cover.jpg"}
	rs := NewResourceSet([]*Resource{r}, nil, nil)
	if _, ok := rs.GetByHref("OEBPS/./images/../images/cover.jpg"); !ok {
		t.Errorf("GetByHref should clean path")
	}
	if r.ResolvedHref != "OEBPS/images/cover.jpg" {
		t.Errorf("ResolvedHref should be canonical")
	}
}

func TestResourceSet_CoverImage(t *testing.T) {
	cover := &Resource{Id: "cover", ResolvedHref: "cover.jpg"}
	r1 := &Resource{Id: "a", ResolvedHref: "a.html"}
	rs := NewResourceSet([]*Resource{r1}, nil, cover)
	c, ok := rs.CoverImage()
	if !ok || c != cover {
		t.Fatalf("CoverImage should return cover")
	}
	if _, ok := rs.GetByID("cover"); !ok {
		t.Fatalf("cover should be indexed by ID")
	}
	if _, ok := rs.GetByHref("cover.jpg"); !ok {
		t.Fatalf("cover should be indexed by href")
	}
}
