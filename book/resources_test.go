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

func TestNewResourceSet_DuplicateHref(t *testing.T) {
	r1 := &Resource{Id: "res1", ResolvedHref: "chapter1.html"}
	r2 := &Resource{Id: "res2", ResolvedHref: "chapter1.html"} // duplicate href
	ro := []ReadingOrderItem{{Resource: r2, Linear: true}}

	rs := NewResourceSet([]*Resource{r1, r2}, ro, nil)

	// First wins: All has length 1
	if len(rs.All()) != 1 {
		t.Fatalf("All() len = %d, want 1", len(rs.All()))
	}
	// Second Id aliases to first resource
	found, ok := rs.GetByID("res2")
	if !ok || found != r1 {
		t.Errorf("GetByID(res2) should alias to res1, got %v", found)
	}
	// ReadingOrder points to deduped resource
	if rs.ReadingOrder()[0].Resource != r1 {
		t.Errorf("ReadingOrder resource should point to deduped r1")
	}
}

func TestNewResourceSet_DuplicateID_DifferentHref(t *testing.T) {
	r1 := &Resource{Id: "item", ResolvedHref: "ch1.html"}
	r2 := &Resource{Id: "item", ResolvedHref: "ch2.html"} // same Id, different href

	rs := NewResourceSet([]*Resource{r1, r2}, nil, nil)
	if len(rs.All()) != 2 {
		t.Fatalf("All() len = %d, want 2", len(rs.All()))
	}

	// r2 gets renamed ID "item__dup1"
	r2Dup, ok := rs.GetByID("item__dup1")
	if !ok || r2Dup.ResolvedHref != "ch2.html" {
		t.Errorf("expected item__dup1 with ch2.html, got %v", r2Dup)
	}
}

func TestNewResourceSet_CoverDuplicatesExisting(t *testing.T) {
	r1 := &Resource{Id: "img1", ResolvedHref: "cover.jpg"}
	cover := &Resource{Id: "cover_res", ResolvedHref: "cover.jpg"} // duplicate href

	rs := NewResourceSet([]*Resource{r1}, nil, cover)
	cov, ok := rs.CoverImage()
	if !ok || cov != r1 {
		t.Errorf("CoverImage should alias to r1, got %v", cov)
	}

	// Cover with same ID but different Href
	coverSameID := &Resource{Id: "img1", ResolvedHref: "cover2.jpg"}
	rs2 := NewResourceSet([]*Resource{r1}, nil, coverSameID)
	cov2, ok := rs2.CoverImage()
	if !ok || cov2.Id != "img1__dup1" {
		t.Errorf("CoverImage with duplicate ID should be renamed, got %v", cov2)
	}
}

