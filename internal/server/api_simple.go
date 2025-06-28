package server

import (
	"parental-control/internal/models"
)

// SimpleAPIServer handles simple API endpoints.

type SimpleAPIServer struct {
	repos *models.RepositoryManager
}

// NewSimpleAPIServer creates a new SimpleAPIServer.
func NewSimpleAPIServer(repoManager *models.RepositoryManager) *SimpleAPIServer {
	return &SimpleAPIServer{
		repos: repoManager,
	}
}

// RegisterRoutes registers the simple API routes with the server.
func (s *SimpleAPIServer) RegisterRoutes(server *Server) {
	// Your route registration logic here
}
