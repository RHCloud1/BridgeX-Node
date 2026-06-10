package service

import (
	"sort"
	"sync"

	"nodebridge/internal/domain"
)

type Registry struct {
	mu    sync.RWMutex
	nodes map[string]domain.Node
}

func NewRegistry() *Registry {
	return &Registry{nodes: make(map[string]domain.Node)}
}

func (r *Registry) ReplaceSource(source string, nodes []domain.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, node := range r.nodes {
		if node.Source.Name == source {
			delete(r.nodes, id)
		}
	}
	for _, node := range nodes {
		r.nodes[node.ID] = node
	}
}

func (r *Registry) UpsertMany(nodes []domain.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, node := range nodes {
		r.nodes[node.ID] = node
	}
}

func (r *Registry) List() []domain.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	nodes := make([]domain.Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Source.Name == nodes[j].Source.Name {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].Source.Name < nodes[j].Source.Name
	})
	return nodes
}
