import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  AppBar,
  Box,
  Container,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
  Button,
  useMediaQuery,
  useTheme,
  SwipeableDrawer,
  Divider,
  Avatar,
} from '@mui/material';
import {
  Dashboard,
  List as ListIcon,
  Security,
  Settings,
  ExitToApp,
  Menu as MenuIcon,
  Close as CloseIcon,
  AdminPanelSettings,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';

interface LayoutProps {
  children: React.ReactNode;
}

const drawerWidth = 280;

const navigationItems = [
  { text: 'Dashboard', icon: <Dashboard />, path: '/dashboard', description: 'System overview and statistics' },
  { text: 'Lists & Rules', icon: <ListIcon />, path: '/lists', description: 'Manage applications and websites' },
  { text: 'Audit Logs', icon: <Security />, path: '/audit', description: 'View system activity logs' },
  { text: 'Configuration', icon: <Settings />, path: '/config', description: 'System settings and preferences' },
];

function Layout({ children }: LayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { logout } = useAuth();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));
  const [mobileOpen, setMobileOpen] = useState(false);

  const handleNavigation = (path: string): void => {
    navigate(path);
    if (isMobile) {
      setMobileOpen(false);
    }
  };

  const handleLogout = async (): Promise<void> => {
    await logout();
  };

  const handleDrawerToggle = (): void => {
    setMobileOpen(!mobileOpen);
  };

  const drawerContent = (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <Box sx={{ p: 2, display: 'flex', alignItems: 'center', minHeight: 64 }}>
        <Avatar sx={{ bgcolor: 'primary.main', mr: 2 }}>
          <AdminPanelSettings />
        </Avatar>
        <Box sx={{ flexGrow: 1 }}>
          <Typography variant="h6" noWrap sx={{ fontWeight: 600 }}>
            Parental Control
          </Typography>
          <Typography variant="body2" color="text.secondary" noWrap>
            Management Console
          </Typography>
        </Box>
        {isMobile && (
          <IconButton 
            onClick={handleDrawerToggle}
            sx={{ ml: 1 }}
          >
            <CloseIcon />
          </IconButton>
        )}
      </Box>
      
      <Divider />
      
      {/* Navigation */}
      <Box sx={{ flexGrow: 1, overflow: 'auto' }}>
        <List sx={{ px: 1, py: 2 }}>
          {navigationItems.map((item) => (
            <ListItem key={item.text} disablePadding sx={{ mb: 0.5 }}>
              <ListItemButton
                selected={location.pathname === item.path}
                onClick={() => handleNavigation(item.path)}
                sx={{
                  borderRadius: 2,
                  mx: 1,
                  '&.Mui-selected': {
                    bgcolor: 'primary.main',
                    color: 'primary.contrastText',
                    '& .MuiListItemIcon-root': {
                      color: 'primary.contrastText',
                    },
                    '&:hover': {
                      bgcolor: 'primary.dark',
                    },
                  },
                  '&:hover': {
                    bgcolor: 'action.hover',
                  },
                }}
              >
                <ListItemIcon 
                  sx={{ 
                    minWidth: 40,
                    color: location.pathname === item.path ? 'inherit' : 'action.active'
                  }}
                >
                  {item.icon}
                </ListItemIcon>
                <ListItemText 
                  primary={item.text}
                  secondary={isMobile ? undefined : item.description}
                  secondaryTypographyProps={{
                    variant: 'caption',
                    sx: { 
                      display: { xs: 'none', md: 'block' },
                      opacity: 0.7,
                    }
                  }}
                />
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      </Box>
      
      <Divider />
      
      {/* Footer */}
      <Box sx={{ p: 2 }}>
        <Button
          fullWidth
          variant={isMobile ? "contained" : "outlined"}
          color="error"
          onClick={handleLogout}
          startIcon={<ExitToApp />}
          sx={{ 
            borderRadius: 2,
            textTransform: 'none',
          }}
        >
          Logout
        </Button>
      </Box>
    </Box>
  );

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      {/* Mobile App Bar */}
      <AppBar
        position="fixed"
        sx={{
          display: { md: 'none' },
          zIndex: theme.zIndex.appBar,
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            aria-label="open drawer"
            edge="start"
            onClick={handleDrawerToggle}
            sx={{ mr: 2 }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            Parental Control
          </Typography>
          <IconButton color="inherit" onClick={handleLogout}>
            <ExitToApp />
          </IconButton>
        </Toolbar>
      </AppBar>

      {/* Desktop App Bar */}
      <AppBar
        position="fixed"
        sx={{
          display: { xs: 'none', md: 'block' },
          width: `calc(100% - ${drawerWidth}px)`,
          ml: `${drawerWidth}px`,
          zIndex: theme.zIndex.appBar,
        }}
      >
        <Toolbar>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            {navigationItems.find(item => item.path === location.pathname)?.text || 'Parental Control Management'}
          </Typography>
          <Button 
            color="inherit" 
            onClick={handleLogout} 
            startIcon={<ExitToApp />}
            sx={{ textTransform: 'none' }}
          >
            Logout
          </Button>
        </Toolbar>
      </AppBar>

      {/* Mobile Drawer */}
      <SwipeableDrawer
        variant="temporary"
        open={mobileOpen}
        onOpen={handleDrawerToggle}
        onClose={handleDrawerToggle}
        ModalProps={{
          keepMounted: true,
        }}
        sx={{
          display: { xs: 'block', md: 'none' },
          zIndex: theme.zIndex.modal, // Higher than app bar
          '& .MuiDrawer-paper': {
            width: drawerWidth,
            maxWidth: '85vw',
          },
        }}
      >
        {drawerContent}
      </SwipeableDrawer>

      {/* Desktop Drawer */}
      <Drawer
        variant="permanent"
        sx={{
          display: { xs: 'none', md: 'block' },
          width: drawerWidth,
          flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: drawerWidth,
            boxSizing: 'border-box',
            borderRight: '1px solid',
            borderColor: 'divider',
          },
        }}
        open
      >
        {drawerContent}
      </Drawer>

      {/* Main Content */}
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          bgcolor: 'background.default',
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          width: { md: `calc(100% - ${drawerWidth}px)` },
        }}
      >
        {/* Spacer for fixed app bar */}
        <Toolbar />
        
        {/* Content Area */}
        <Container 
          maxWidth="xl" 
          sx={{ 
            flexGrow: 1,
            py: { xs: 2, sm: 3 },
            px: { xs: 2, sm: 3 },
          }}
        >
          {children}
        </Container>
      </Box>
    </Box>
  );
}

export default Layout; 