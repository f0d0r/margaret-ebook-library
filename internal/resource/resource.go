package resource

import (
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

// NewResourceSet builds a model.ResourceSet from the given slices.
// See model.NewResourceSet for full semantics. This is the canonical
// implementation; pkg/model delegates here.
func NewResourceSet(all []*model.Resource, readingOrder []model.ReadingOrderItem, cover *model.Resource) *model.ResourceSet {
	filteredAll := make([]*model.Resource, 0, len(all))
	for _, r := range all {
		if r == nil {
			continue
		}
		filteredAll = append(filteredAll, r)
	}
	filteredRO := make([]model.ReadingOrderItem, 0, len(readingOrder))
	for _, it := range readingOrder {
		if it.Resource == nil {
			continue
		}
		filteredRO = append(filteredRO, it)
	}

	byID := make(map[string]*model.Resource)
	byHref := make(map[string]*model.Resource)
	seenHref := make(map[string]*model.Resource)
	allCopy := make([]*model.Resource, 0, len(filteredAll))

	for _, orig := range filteredAll {
		hrefKey := ""
		if orig.ResolvedHref != "" {
			hrefKey = util.CleanHref(orig.ResolvedHref)
		}
		if hrefKey != "" {
			if existing, ok := seenHref[hrefKey]; ok {
				if orig.Id != "" {
					if _, exists := byID[orig.Id]; !exists {
						byID[orig.Id] = existing
					}
				}
				continue
			}
		}
		r := orig
		if r.Id != "" {
			if _, exists := byID[r.Id]; exists {
				var newID string
				for i := 1; ; i++ {
					cand := fmt.Sprintf("%s__dup%d", r.Id, i)
					if _, exists2 := byID[cand]; !exists2 {
						newID = cand
						break
					}
				}
				copyRes := *r
				copyRes.Id = newID
				r = &copyRes
				if hrefKey != "" {
					hrefKey = util.CleanHref(r.ResolvedHref)
				}
			}
		}
		if hrefKey != "" {
			seenHref[hrefKey] = r
			byHref[hrefKey] = r
		}
		if r.Id != "" {
			byID[r.Id] = r
		}
		allCopy = append(allCopy, r)
	}

	roCopy := make([]model.ReadingOrderItem, 0, len(filteredRO))
	for _, it := range filteredRO {
		res := it.Resource
		if res.ResolvedHref != "" {
			if deduped, ok := seenHref[util.CleanHref(res.ResolvedHref)]; ok {
				res = deduped
			}
		}
		roCopy = append(roCopy, model.ReadingOrderItem{Resource: res, Linear: it.Linear})
	}

	coverAliased := cover
	if cover != nil && cover.ResolvedHref != "" {
		if deduped, ok := seenHref[util.CleanHref(cover.ResolvedHref)]; ok {
			coverAliased = deduped
		} else {
			if cover.Id != "" {
				if _, exists := byID[cover.Id]; exists {
					var newID string
					for i := 1; ; i++ {
						cand := fmt.Sprintf("%s__dup%d", cover.Id, i)
						if _, exists2 := byID[cand]; !exists2 {
							newID = cand
							break
						}
					}
					copyCover := *cover
					copyCover.Id = newID
					coverAliased = &copyCover
				} else {
					byID[cover.Id] = coverAliased
				}
			}
			if _, exists := byHref[util.CleanHref(cover.ResolvedHref)]; !exists {
				byHref[util.CleanHref(cover.ResolvedHref)] = coverAliased
			}
		}
	} else if cover != nil && cover.Id != "" {
		if _, exists := byID[cover.Id]; !exists {
			byID[cover.Id] = coverAliased
		}
	}

	return model.NewResourceSetFromParts(allCopy, byID, byHref, roCopy, coverAliased)
}
