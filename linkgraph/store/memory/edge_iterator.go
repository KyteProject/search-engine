package memory

import "github.com/kyteproject/search-engine/linkgraph/graph"

// edgeIterator is a graph.EdgeIterator implementation for the in-memory graph.
type edgeIterator struct {
	s *InMemoryGraph

	edges    []*graph.Edge
	curIndex int
}

// Next implements graph.EdgeIterator.
func (i *edgeIterator) Next() bool {
	if i.curIndex >= len(i.edges) {
		return false
	}
	i.curIndex++
	return true
}

// Edge implements graph.EdgeIterator.
func (i edgeIterator) Edge() *graph.Edge {
	// The edge pointer contents may be overwritten by a graph update; to
	// avoid data-races we acquire the read lock first and clone the edge
	i.s.mu.RLock()
	edge := new(graph.Edge)
	*edge = *i.edges[i.curIndex-1]
	i.s.mu.RUnlock()
	return edge
}

// Error implements graph.EdgeIterator.
func (i *edgeIterator) Error() error {
	return nil
}

// Close implements graph.EdgeIterator.
func (i *edgeIterator) Close() error {
	return nil
}
