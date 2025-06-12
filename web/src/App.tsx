import { useState, useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';

// Import components (will be created in subsequent tasks)
import LoginPage from './pages/LoginPage';
import Dashboard from './pages/Dashboard';
import ListsPage from './pages/ListsPage';
import AuditPage from './pages/AuditPage';
import ConfigPage from './pages/ConfigPage';
import Layout from './components/Layout';

// Import API client
import { apiClient } from './services/api';

interface AppState {
  isAuthenticated: boolean;
  isLoading: boolean;
}

function App() {
  const [appState, setAppState] = useState<AppState>({
    isAuthenticated: false,
    isLoading: true,
  });

  useEffect(() => {
    // Check authentication status on app startup
    const checkAuth = async (): Promise<void> => {
      try {
        const isAuth = await apiClient.checkAuth();
        setAppState({
          isAuthenticated: isAuth,
          isLoading: false,
        });
      } catch {
        setAppState({
          isAuthenticated: false,
          isLoading: false,
        });
      }
    };

    void checkAuth();
  }, []);

  const handleLogin = (): void => {
    setAppState(prev => ({
      ...prev,
      isAuthenticated: true,
    }));
  };

  const handleLogout = (): void => {
    setAppState(prev => ({
      ...prev,
      isAuthenticated: false,
    }));
    void apiClient.logout();
  };

  // Show loading spinner while checking authentication
  if (appState.isLoading) {
    return (
      <Box 
        display="flex" 
        justifyContent="center" 
        alignItems="center" 
        minHeight="100vh"
      >
        <CircularProgress />
      </Box>
    );
  }

  // If not authenticated, show login page
  if (!appState.isAuthenticated) {
    return <LoginPage onLogin={handleLogin} />;
  }

  // Main application with authenticated routes
  return (
    <Layout onLogout={handleLogout}>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/lists" element={<ListsPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/config" element={<ConfigPage />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Layout>
  );
}

export default App; 