package opf

import (
	"encoding/xml"
	"strings"
	"testing"
)

// emptyPackage returns a minimal modern package without metas or manifest
// items, for not-found test cases.
func emptyPackage() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
</package>`
}

func TestGetMetaByName(t *testing.T) {
	tests := []struct {
		name         string
		opfContent   string
		searchName   string
		expectedMeta *Meta
		shouldFind   bool
	}{
		{
			name:       "Meta with matching name",
			opfContent: createOPFWithMeta("calibre", "content-value"),
			searchName: "calibre",
			expectedMeta: &Meta{
				Name:    "calibre",
				Content: "content-value",
			},
			shouldFind: true,
		},
		{
			name:       "Meta with matching name and property",
			opfContent: createOPFWithMetaProperty("identifier-type", "uuid", "uuid:12345"),
			searchName: "identifier-type",
			expectedMeta: &Meta{
				Name:     "identifier-type",
				Property: "uuid",
				Content:  "uuid:12345",
			},
			shouldFind: true,
		},
		{
			name:         "Meta not found",
			opfContent:   createOPFWithMeta("calibre", "value"),
			searchName:   "non-existent",
			expectedMeta: nil,
			shouldFind:   false,
		},
		{
			name:         "Empty metadata - no metas",
			opfContent:   emptyPackage(),
			searchName:   "calibre",
			expectedMeta: nil,
			shouldFind:   false,
		},
		{
			name: "Multiple metas - find specific one",
			opfContent: createOPFWithMultipleMetas([]MetaData{
				{Name: "meta1", Content: "value1"},
				{Name: "meta2", Content: "value2"},
				{Name: "meta3", Content: "value3"},
			}),
			searchName: "meta2",
			expectedMeta: &Meta{
				Name:    "meta2",
				Content: "value2",
			},
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			result := pkg.MetaByName(tt.searchName)

			if tt.shouldFind {
				if result == nil {
					t.Errorf("expected to find meta with name %q, but got nil", tt.searchName)
					return
				}
				if result.Name != tt.expectedMeta.Name {
					t.Errorf("got meta name %q, want %q", result.Name, tt.expectedMeta.Name)
				}
				if result.Content != tt.expectedMeta.Content {
					t.Errorf("got meta content %q, want %q", result.Content, tt.expectedMeta.Content)
				}
				if tt.expectedMeta.Property != "" && result.Property != tt.expectedMeta.Property {
					t.Errorf("got meta property %q, want %q", result.Property, tt.expectedMeta.Property)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil, but got meta: %+v", result)
				}
			}
		})
	}
}

func TestGetItemById(t *testing.T) {
	tests := []struct {
		name         string
		opfContent   string
		searchID     string
		expectedItem *Item
		shouldFind   bool
	}{
		{
			name:       "Item with matching ID",
			opfContent: createOPFWithManifestItem("item1", "text/html", "chapter1.html", ""),
			searchID:   "item1",
			expectedItem: &Item{
				ID:        "item1",
				MediaType: "text/html",
				Href:      "chapter1.html",
			},
			shouldFind: true,
		},
		{
			name:       "Item with matching ID and properties",
			opfContent: createOPFWithManifestItem("ncx", "application/x-dtbncx+xml", "toc.ncx", "nav"),
			searchID:   "ncx",
			expectedItem: &Item{
				ID:         "ncx",
				MediaType:  "application/x-dtbncx+xml",
				Href:       "toc.ncx",
				Properties: "nav",
			},
			shouldFind: true,
		},
		{
			name:         "Item not found",
			opfContent:   createOPFWithManifestItem("item1", "text/html", "chapter1.html", ""),
			searchID:     "non-existent",
			expectedItem: nil,
			shouldFind:   false,
		},
		{
			name:         "Empty manifest - no items",
			opfContent:   emptyPackage(),
			searchID:     "item1",
			expectedItem: nil,
			shouldFind:   false,
		},
		{
			name: "Multiple items - find specific one",
			opfContent: createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: ""},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
				{ID: "item3", MediaType: "image/jpeg", Href: "image.jpg", Properties: ""},
			}),
			searchID: "item2",
			expectedItem: &Item{
				ID:        "item2",
				MediaType: "text/html",
				Href:      "chapter2.html",
			},
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			result := pkg.ItemById(tt.searchID)

			if tt.shouldFind {
				if result == nil {
					t.Errorf("expected to find item with ID %q, but got nil", tt.searchID)
					return
				}
				if result.ID != tt.expectedItem.ID {
					t.Errorf("got item ID %q, want %q", result.ID, tt.expectedItem.ID)
				}
				if result.MediaType != tt.expectedItem.MediaType {
					t.Errorf("got item media type %q, want %q", result.MediaType, tt.expectedItem.MediaType)
				}
				if result.Href != tt.expectedItem.Href {
					t.Errorf("got item href %q, want %q", result.Href, tt.expectedItem.Href)
				}
				if tt.expectedItem.Properties != "" && result.Properties != tt.expectedItem.Properties {
					t.Errorf("got item properties %q, want %q", result.Properties, tt.expectedItem.Properties)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil, but got item: %+v", result)
				}
			}
		})
	}
}

func TestGetItemsByProperty(t *testing.T) {
	tests := []struct {
		name           string
		opfContent     string
		searchProperty string
		expectedItems  []Item
		expectedCount  int
	}{
		{
			name: "Items with matching property",
			opfContent: createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: "svg"},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
				{ID: "item3", MediaType: "image/svg+xml", Href: "image.svg", Properties: "svg"},
			}),
			searchProperty: "svg",
			expectedCount:  2,
		},
		{
			name:           "Single item with property",
			opfContent:     createOPFWithManifestItem("cover", "image/jpeg", "cover.jpg", "cover-image"),
			searchProperty: "cover-image",
			expectedCount:  1,
		},
		{
			name: "Property with space in properties",
			opfContent: createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: "scripted remote-resources"},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: "remote-resources"},
				{ID: "item3", MediaType: "text/html", Href: "chapter3.html", Properties: "scripted"},
			}),
			searchProperty: "remote-resources",
			expectedCount:  2,
		},
		{
			name: "No items with property",
			opfContent: createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: ""},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
			}),
			searchProperty: "nav",
			expectedCount:  0,
		},
		{
			name:           "Empty manifest",
			opfContent:     emptyPackage(),
			searchProperty: "nav",
			expectedCount:  0,
		},
		{
			name:           "Property search is case-sensitive",
			opfContent:     createOPFWithManifestItem("item1", "text/html", "chapter1.html", "NAV"),
			searchProperty: "nav",
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			results := pkg.ItemsByProperty(tt.searchProperty)

			if len(results) != tt.expectedCount {
				t.Errorf("got %d items with property %q, want %d", len(results), tt.searchProperty, tt.expectedCount)
				return
			}

			for _, item := range results {
				if !strings.Contains(item.Properties, tt.searchProperty) {
					t.Errorf("item %q has properties %q, but doesn't contain %q", item.ID, item.Properties, tt.searchProperty)
				}
			}
		})
	}
}

// Helper types and functions for tests
type MetaData struct {
	Name     string
	Property string
	Content  string
}

type ItemData struct {
	ID         string
	MediaType  string
	Href       string
	Properties string
}

func parseOPFContent(t *testing.T, opfContent string) Package {
	var pkg Package
	if err := xml.Unmarshal([]byte(opfContent), &pkg); err != nil {
		t.Fatalf("failed to unmarshal OPF content: %v", err)
	}
	return pkg
}

func createOPFWithMeta(name, content string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="` + name + `" content="` + content + `" />
  </metadata>
</package>`
}

func createOPFWithMetaProperty(name, property, content string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="` + name + `" property="` + property + `" content="` + content + `" />
  </metadata>
</package>`
}

func createOPFWithMultipleMetas(metas []MetaData) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, meta := range metas {
		if meta.Property != "" {
			metadataContent += "\n    <meta name=\"" + meta.Name + "\" property=\"" + meta.Property + "\" content=\"" + meta.Content + "\" />"
		} else {
			metadataContent += "\n    <meta name=\"" + meta.Name + "\" content=\"" + meta.Content + "\" />"
		}
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithManifestItem(id, mediaType, href, properties string) string {
	propertiesAttr := ""
	if properties != "" {
		propertiesAttr = ` properties="` + properties + `"`
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
  <manifest>
    <item id="` + id + `" media-type="` + mediaType + `" href="` + href + `"` + propertiesAttr + ` />
  </manifest>
</package>`
}

func createOPFWithMultipleItems(items []ItemData) string {
	manifestContent := `  <manifest>`
	for _, item := range items {
		propertiesAttr := ""
		if item.Properties != "" {
			propertiesAttr = ` properties="` + item.Properties + `"`
		}
		manifestContent += "\n    <item id=\"" + item.ID + "\" media-type=\"" + item.MediaType + "\" href=\"" + item.Href + "\"" + propertiesAttr + " />"
	}
	manifestContent += "\n  </manifest>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
` + manifestContent + `
</package>`
}
