package memory

import (
	"github.com/google/uuid"
	"github.com/kyteproject/search-engine/linkgraph/graph"
	"golang.org/x/xerrors"
	"sync"
	"time"
)

// Compile-time check for ensuring InMemoryGraph implements Graph.
var _ graph.Graph = (*InMemoryGraph)(nil)

// edgeList contains the slice of edge UUIDs that originate from a link in the graph.
type edgeList []uuid.UUID

// InMemoryGraph implements an in-memory link graph that can be concurrently
// accessed by multiple clients.
type InMemoryGraph struct {
	mu sync.RWMutex

	links map[uuid.UUID]*graph.Link
	edges map[uuid.UUID]*graph.Edge

	linkURLIndex map[string]*graph.Link
	linkEdgeMap  map[uuid.UUID]edgeList
}

// NewInMemoryGraph creates a new in-memory link graph.
func NewInMemoryGraph() *InMemoryGraph {
	return &InMemoryGraph{
		links:        make(map[uuid.UUID]*graph.Link),
		edges:        make(map[uuid.UUID]*graph.Edge),
		linkURLIndex: make(map[string]*graph.Link),
		linkEdgeMap:  make(map[uuid.UUID]edgeList),
	}
}

// UpsertLink creates a new link or updates and existing link.
func (s *InMemoryGraph) UpsertLink(link *graph.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if a link with the same URL already exists. If so, convert
	// this into an update and point the link ID to the existing link
	// while retaining the most recent RetrievedAt timestamp.
	if existing := s.linkURLIndex[link.URL]; existing != nil {
		link.ID = existing.ID
		origTimestamp := existing.RetrievedAt
		*existing = *link
		if origTimestamp.After(existing.RetrievedAt) {
			existing.RetrievedAt = origTimestamp
		}
		return nil
	}

	// Assign new ID.
	for {
		link.ID = uuid.New()
		if s.links[link.ID] == nil {
			break
		}
	}

	// Make copy and insert link into map structure.
	lCopy := new(graph.Link)
	*lCopy = *link
	s.linkURLIndex[lCopy.URL] = lCopy
	s.links[lCopy.ID] = lCopy
	return nil
}

// UpsertEdge creates a new edge or updates an existing edge.
func (s *InMemoryGraph) UpsertEdge(edge *graph.Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify source and destination links exist
	_, sourceExists := s.links[edge.Source]
	_, destinationExists := s.links[edge.Destination]
	if !sourceExists || !destinationExists {
		return xerrors.Errorf("upsert edge: %w", graph.ErrUnknownEdgeLinks)
	}

	// Scan edge list from source
	for _, edgeID := range s.linkEdgeMap[edge.Source] {
		existingEdge := s.edges[edgeID]
		if existingEdge.Source == edge.Source && existingEdge.Destination == edge.Destination {
			existingEdge.UpdatedAt = time.Now()
			*edge = *existingEdge
			return nil
		}
	}

	// Insert new edge
	for {
		edge.ID = uuid.New()
		if s.edges[edge.ID] == nil {
			break
		}
	}

	// Make copy
	edge.UpdatedAt = time.Now()
	eCopy := new(graph.Edge)
	*eCopy = *edge
	s.edges[eCopy.ID] = eCopy

	// Append the edge ID to the list of edges originating from the
	// edge's source link.
	s.linkEdgeMap[edge.Source] = append(s.linkEdgeMap[edge.Source], eCopy.ID)
	return nil
}

// FindLink looks up a link by ID and returns a copy of the link stored in graph.
func (s *InMemoryGraph) FindLink(id uuid.UUID) (*graph.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link := s.links[id]
	if link == nil {
		return nil, xerrors.Errorf("find link: %w", graph.ErrNotFound)
	}

	lCopy := new(graph.Link)
	*lCopy = *link
	return lCopy, nil
}

// Links returns an iterator for the set of links whose IDs belong to the
// [fromID, toID] range and were retrieved before the provided timestamp.
func (s *InMemoryGraph) Links(fromID, toID uuid.UUID, retrievedBefore time.Time) (graph.LinkIterator, error) {
	from, to := fromID.String(), toID.String()

	s.mu.RLock()
	var list []*graph.Link
	for linkID, link := range s.links {
		if id := linkID.String(); id >= from && id < to && link.RetrievedAt.Before(retrievedBefore) {
			list = append(list, link)
		}
	}
	s.mu.RUnlock()

	return &linkIterator{s: s, links: list}, nil
}

// Edges returns an iterator for the set of edges whose source vertex IDs
// belong to the [fromID, toID) range and were updated before the provided
// timestamp.
func (s *InMemoryGraph) Edges(fromID, toID uuid.UUID, updatedBefore time.Time) (graph.EdgeIterator, error) {
	from, to := fromID.String(), toID.String()

	s.mu.RLock()

	// Iterate links in the graph
	var list []*graph.Edge
	for linkID := range s.links {
		// Skip links that do not belong to the partition we need
		if id := linkID.String(); id < from || id >= to {
			continue
		}

		// Iterate the list of edges (via the linkEdgeMap field)
		for _, edgeID := range s.linkEdgeMap[linkID] {
			// append edges that satisfy the updated-before-X predicate
			if edge := s.edges[edgeID]; edge.UpdatedAt.Before(updatedBefore) {
				list = append(list, edge)
			}
		}
	}
	s.mu.RUnlock()

	return &edgeIterator{s: s, edges: list}, nil
}

// RemoveStaleEdges removes any edge that originates from the specified link ID
// and was updated before the specified timestamp.
func (s *InMemoryGraph) RemoveStaleEdges(fromID uuid.UUID, updatedBefore time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Iterate list of edges that originate from the specified source link
	var newEdgeList edgeList
	for _, edgeID := range s.linkEdgeMap[fromID] {
		edge := s.edges[edgeID]
		if edge.UpdatedAt.Before(updatedBefore) {
			delete(s.edges, edgeID)
			continue
		}

		newEdgeList = append(newEdgeList, edgeID)
	}

	// Replace edge list or origin link with the filtered edge list
	s.linkEdgeMap[fromID] = newEdgeList
	return nil
}
