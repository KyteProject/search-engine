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
