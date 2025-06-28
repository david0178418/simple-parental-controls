package server

import (
	"parental-control/internal/models"
)

// DashboardAPIServer handles dashboard-related API endpoints.

type DashboardAPIServer struct {
	repos *models.RepositoryManager
}

// NewDashboardAPIServer creates a new DashboardAPIServer.
func NewDashboardAPIServer(repoManager *models.RepositoryManager) *DashboardAPIServer {
	return &DashboardAPIServer{
		repos: repoManager,
	}
}

// RegisterRoutes registers the dashboard API routes with the server.
func (s *DashboardAPIServer) RegisterRoutes(server *Server) {
	// Your route registration logic here
}
