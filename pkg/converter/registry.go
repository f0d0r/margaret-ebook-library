package converter

import (
	"sync"

	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

type edge struct {
	from string
	to   string
}

// Registry holds MIME-to-MIME transformers.
type Registry struct {
	mu     sync.RWMutex
	byEdge map[edge]Transformer
}

// NewRegistry creates a registry pre-populated with the given transformers.
func NewRegistry(transformers ...Transformer) *Registry {
	r := &Registry{byEdge: make(map[edge]Transformer)}
	for _, t := range transformers {
		r.Register(t)
	}
	return r
}

// Register adds or replaces a transformer for its From()->To() edge.
func (r *Registry) Register(t Transformer) {
	if t == nil {
		return
	}
	from := mediatype.Normalize(t.From())
	to := mediatype.Normalize(t.To())
	if from == "" || to == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byEdge == nil {
		r.byEdge = make(map[edge]Transformer)
	}
	r.byEdge[edge{from: from, to: to}] = t
}

// Find returns a direct transformer for from->to, if any.
func (r *Registry) Find(from, to string) (Transformer, bool) {
	from = mediatype.Normalize(from)
	to = mediatype.Normalize(to)
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byEdge[edge{from: from, to: to}]
	return t, ok
}

// FindPath returns the shortest transformer chain from->to via BFS.
// For now the graph is small; BFS is cheap and handles future multi-hop
// conversions (e.g. x-mobipocket-html -> xhtml -> plain).
func (r *Registry) FindPath(from, to string) ([]Transformer, bool) {
	from = mediatype.Normalize(from)
	to = mediatype.Normalize(to)
	if from == "" || to == "" {
		return nil, false
	}
	if from == to {
		return nil, true // identity, no transformer needed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.byEdge) == 0 {
		return nil, false
	}

	// BFS over MIME nodes.
	type node struct {
		mime string
		path []Transformer
	}
	visited := map[string]bool{from: true}
	queue := []node{{mime: from}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for e, t := range r.byEdge {
			if e.from != cur.mime || visited[e.to] {
				continue
			}
			newPath := append(append([]Transformer(nil), cur.path...), t)
			if e.to == to {
				return newPath, true
			}
			visited[e.to] = true
			queue = append(queue, node{mime: e.to, path: newPath})
		}
	}
	return nil, false
}

// RegisterAliases registers the same transformer logic for multiple source
// MIME types targeting a single destination. The factory is called once per
// source type so each edge gets its own Transformer instance.
func RegisterAliases(reg *Registry, factory func(from, to string) Transformer, to string, froms ...string) {
	if reg == nil || factory == nil {
		return
	}
	for _, f := range froms {
		reg.Register(factory(f, to))
	}
}
