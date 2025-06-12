import { Routes, Route, Navigate } from 'react-router-dom';

// Import components
import LoginPage from './pages/LoginPage';
import Dashboard from './pages/Dashboard';
import ListsPage from './pages/ListsPage';
import AuditPage from './pages/AuditPage';
import ConfigPage from './pages/ConfigPage';
import Layout from './components/Layout';
import ProtectedRoute from './components/ProtectedRoute';
import { AuthProvider } from './contexts/AuthContext';

function App() {
  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        
        {/* Protected routes with layout */}
        <Route path="/" element={
          <ProtectedRoute>
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        } />
        
        <Route path="/dashboard" element={
          <ProtectedRoute>
            <Layout>
              <Dashboard />
            </Layout>
          </ProtectedRoute>
        } />
        
        <Route path="/lists" element={
          <ProtectedRoute>
            <Layout>
              <ListsPage />
            </Layout>
          </ProtectedRoute>
        } />
        
        <Route path="/audit" element={
          <ProtectedRoute>
            <Layout>
              <AuditPage />
            </Layout>
          </ProtectedRoute>
        } />
        
        <Route path="/config" element={
          <ProtectedRoute>
            <Layout>
              <ConfigPage />
            </Layout>
          </ProtectedRoute>
        } />
        
        {/* Fallback route */}
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AuthProvider>
  );
}

export default App; 