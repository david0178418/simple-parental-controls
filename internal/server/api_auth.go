package server

import (
	"parental-control/internal/models"
)

// AuthAPIServer handles authentication-related API endpoints.

type AuthAPIServer struct {
	repos          *models.RepositoryManager
	authMiddleware *AuthMiddleware
}

// NewAuthAPIServer creates a new AuthAPIServer.
func NewAuthAPIServer(repoManager *models.RepositoryManager, authMiddleware *AuthMiddleware) *AuthAPIServer {
	return &AuthAPIServer{
		repos:          repoManager,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers the authentication API routes with the server.
func (s *AuthAPIServer) RegisterRoutes(server *Server) {
	// Your route registration logic here
}
