import React, { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Tabs,
  Tab,
  Grid,
  Card,
  CardContent,
  CardHeader,
  TextField,
  Button,
  Alert,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  CircularProgress,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
} from '@mui/material';
import {
  Settings,
  Security,
  TuneOutlined,
  Edit,
  Save,
  Cancel,
  Refresh,
  Visibility,
  VisibilityOff,
  Warning,
  Info,
} from '@mui/icons-material';
import { apiClient, ApiError } from '../services/api';
import { Config } from '../types/api';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel({ children, value, index }: TabPanelProps) {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`config-tabpanel-${index}`}
      aria-labelledby={`config-tab-${index}`}
    >
      {value === index && <Box sx={{ pt: 3 }}>{children}</Box>}
    </div>
  );
}

interface ConfigFormData {
  key: string;
  value: string;
  description: string;
}

interface PasswordFormData {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
}

function ConfigPage() {
  const [tabValue, setTabValue] = useState(0);
  const [configs, setConfigs] = useState<Config[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [editingConfig, setEditingConfig] = useState<Config | null>(null);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false);
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  
  const [configForm, setConfigForm] = useState<ConfigFormData>({
    key: '',
    value: '',
    description: '',
  });
  
  const [passwordForm, setPasswordForm] = useState<PasswordFormData>({
    oldPassword: '',
    newPassword: '',
    confirmPassword: '',
  });

  // Load configurations
  const loadConfigs = async (): Promise<void> => {
    try {
      setLoading(true);
      setError(null);
      const configsData = await apiClient.getConfigs();
      setConfigs(configsData);
    } catch (err) {
      const errorMsg = err instanceof ApiError ? err.message : 'Failed to load configurations';
      setError(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadConfigs();
  }, []);

  // Clear messages after 5 seconds
  useEffect(() => {
    if (error || success) {
      const timer = setTimeout(() => {
        setError(null);
        setSuccess(null);
      }, 5000);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [error, success]);

  const handleTabChange = (_: React.SyntheticEvent, newValue: number): void => {
    setTabValue(newValue);
  };

  const handleEditConfig = (config: Config): void => {
    setConfigForm({
      key: config.key,
      value: config.value,
      description: config.description,
    });
    setEditingConfig(config);
    setEditDialogOpen(true);
  };

  const handleSaveConfig = async (): Promise<void> => {
    if (!editingConfig) return;
    
    try {
      setLoading(true);
      await apiClient.updateConfig(editingConfig.key, configForm.value);
      await loadConfigs();
      setEditDialogOpen(false);
      setEditingConfig(null);
      setSuccess('Configuration updated successfully');
    } catch (err) {
      const errorMsg = err instanceof ApiError ? err.message : 'Failed to update configuration';
      setError(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const handlePasswordChange = async (): Promise<void> => {
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      setError('New passwords do not match');
      return;
    }
    
    if (passwordForm.newPassword.length < 6) {
      setError('New password must be at least 6 characters long');
      return;
    }
    
    try {
      setLoading(true);
      await apiClient.changePassword(passwordForm.oldPassword, passwordForm.newPassword);
      setPasswordDialogOpen(false);
      setPasswordForm({ oldPassword: '', newPassword: '', confirmPassword: '' });
      setSuccess('Password changed successfully');
    } catch (err) {
      const errorMsg = err instanceof ApiError ? err.message : 'Failed to change password';
      setError(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const togglePasswordVisibility = (field: string): void => {
    setShowPasswords(prev => ({
      ...prev,
      [field]: !prev[field]
    }));
  };

  const getConfigsByCategory = (category: string): Config[] => {
    return configs.filter(c => c.key.startsWith(category));
  };

  const formatConfigKey = (key: string): string => {
    return key.split('.').pop()?.replace(/_/g, ' ').toUpperCase() || key;
  };

  const getConfigIcon = (key: string): React.ReactNode => {
    if (key.includes('security') || key.includes('auth')) return <Security />;
    if (key.includes('log') || key.includes('audit')) return <Info />;
    return <Settings />;
  };

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 3 }}>
        <Settings sx={{ mr: 2, fontSize: 32 }} />
        <Typography variant="h4" gutterBottom sx={{ mb: 0 }}>
          Configuration Management
        </Typography>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      <Paper sx={{ width: '100%' }}>
        <Tabs
          value={tabValue}
          onChange={handleTabChange}
          aria-label="configuration tabs"
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          <Tab
            icon={<Settings />}
            label="System Configuration"
            id="config-tab-0"
            aria-controls="config-tabpanel-0"
          />
          <Tab
            icon={<Security />}
            label="Password & Security"
            id="config-tab-1"
            aria-controls="config-tabpanel-1"
          />
          <Tab
            icon={<TuneOutlined />}
            label="Advanced Options"
            id="config-tab-2"
            aria-controls="config-tabpanel-2"
          />
        </Tabs>

        {/* System Configuration Tab */}
        <TabPanel value={tabValue} index={0}>
          <Box sx={{ p: 3 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
              <Typography variant="h6">System Settings</Typography>
              <Button
                startIcon={<Refresh />}
                onClick={() => void loadConfigs()}
                disabled={loading}
              >
                Refresh
              </Button>
            </Box>

            {loading ? (
              <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
                <CircularProgress />
              </Box>
            ) : (
              <Grid container spacing={3}>
                {/* Core Settings */}
                <Grid size={{ xs: 12, md: 6 }}>
                  <Card>
                    <CardHeader title="Core Settings" />
                    <CardContent>
                      <List>
                        {getConfigsByCategory('system').map((config) => (
                          <ListItem key={config.id}>
                            <Box sx={{ mr: 2 }}>
                              {getConfigIcon(config.key)}
                            </Box>
                            <ListItemText
                              primary={formatConfigKey(config.key)}
                              secondary={
                                <Box>
                                  <Typography variant="body2" color="text.secondary">
                                    {config.description}
                                  </Typography>
                                  <Chip
                                    label={config.value}
                                    size="small"
                                    sx={{ mt: 1 }}
                                    color={config.value === 'true' || config.value === 'enabled' ? 'success' : 'default'}
                                  />
                                </Box>
                              }
                            />
                            <ListItemSecondaryAction>
                              <IconButton
                                edge="end"
                                onClick={() => handleEditConfig(config)}
                              >
                                <Edit />
                              </IconButton>
                            </ListItemSecondaryAction>
                          </ListItem>
                        ))}
                        {getConfigsByCategory('system').length === 0 && (
                          <ListItem>
                            <ListItemText primary="No system configurations found" />
                          </ListItem>
                        )}
                      </List>
                    </CardContent>
                  </Card>
                </Grid>

                {/* Network Settings */}
                <Grid size={{ xs: 12, md: 6 }}>
                  <Card>
                    <CardHeader title="Network & Service Settings" />
                    <CardContent>
                      <List>
                        {getConfigsByCategory('server').concat(getConfigsByCategory('network')).map((config) => (
                          <ListItem key={config.id}>
                            <Box sx={{ mr: 2 }}>
                              {getConfigIcon(config.key)}
                            </Box>
                            <ListItemText
                              primary={formatConfigKey(config.key)}
                              secondary={
                                <Box>
                                  <Typography variant="body2" color="text.secondary">
                                    {config.description}
                                  </Typography>
                                  <Chip
                                    label={config.value}
                                    size="small"
                                    sx={{ mt: 1 }}
                                    color="primary"
                                  />
                                </Box>
                              }
                            />
                            <ListItemSecondaryAction>
                              <IconButton
                                edge="end"
                                onClick={() => handleEditConfig(config)}
                              >
                                <Edit />
                              </IconButton>
                            </ListItemSecondaryAction>
                          </ListItem>
                        ))}
                        {getConfigsByCategory('server').concat(getConfigsByCategory('network')).length === 0 && (
                          <ListItem>
                            <ListItemText primary="No network configurations found" />
                          </ListItem>
                        )}
                      </List>
                    </CardContent>
                  </Card>
                </Grid>
              </Grid>
            )}
          </Box>
        </TabPanel>

        {/* Password & Security Tab */}
        <TabPanel value={tabValue} index={1}>
          <Box sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              Password & Security Management
            </Typography>

            <Grid container spacing={3}>
              {/* Password Change */}
              <Grid size={{ xs: 12, md: 6 }}>
                <Card>
                  <CardHeader title="Change Password" />
                  <CardContent>
                    <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                      Change your administrative password for enhanced security.
                    </Typography>
                    <Button
                      variant="contained"
                      startIcon={<Security />}
                      onClick={() => setPasswordDialogOpen(true)}
                      color="primary"
                    >
                      Change Password
                    </Button>
                  </CardContent>
                </Card>
              </Grid>

              {/* Security Settings */}
              <Grid size={{ xs: 12, md: 6 }}>
                <Card>
                  <CardHeader title="Security Settings" />
                  <CardContent>
                    <List>
                      {getConfigsByCategory('security').concat(getConfigsByCategory('auth')).map((config) => (
                        <ListItem key={config.id}>
                          <Box sx={{ mr: 2 }}>
                            <Security />
                          </Box>
                          <ListItemText
                            primary={formatConfigKey(config.key)}
                            secondary={
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  {config.description}
                                </Typography>
                                <Chip
                                  label={config.value}
                                  size="small"
                                  sx={{ mt: 1 }}
                                  color="secondary"
                                />
                              </Box>
                            }
                          />
                          <ListItemSecondaryAction>
                            <IconButton
                              edge="end"
                              onClick={() => handleEditConfig(config)}
                            >
                              <Edit />
                            </IconButton>
                          </ListItemSecondaryAction>
                        </ListItem>
                      ))}
                      {getConfigsByCategory('security').concat(getConfigsByCategory('auth')).length === 0 && (
                        <ListItem>
                          <ListItemText primary="No security configurations found" />
                        </ListItem>
                      )}
                    </List>
                  </CardContent>
                </Card>
              </Grid>

              {/* Session Management */}
              <Grid size={{ xs: 12 }}>
                <Card>
                  <CardHeader title="Session Management" />
                  <CardContent>
                    <Alert severity="info" icon={<Info />}>
                      Session management controls will be implemented in future updates.
                      Current session: Active and secure.
                    </Alert>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>
          </Box>
        </TabPanel>

        {/* Advanced Options Tab */}
        <TabPanel value={tabValue} index={2}>
          <Box sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              Advanced Configuration Options
            </Typography>

            <Grid container spacing={3}>
              {/* Logging Settings */}
              <Grid size={{ xs: 12, md: 6 }}>
                <Card>
                  <CardHeader title="Logging & Audit Settings" />
                  <CardContent>
                    <List>
                      {getConfigsByCategory('log').concat(getConfigsByCategory('audit')).map((config) => (
                        <ListItem key={config.id}>
                          <Box sx={{ mr: 2 }}>
                            <Info />
                          </Box>
                          <ListItemText
                            primary={formatConfigKey(config.key)}
                            secondary={
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  {config.description}
                                </Typography>
                                <Chip
                                  label={config.value}
                                  size="small"
                                  sx={{ mt: 1 }}
                                  color="info"
                                />
                              </Box>
                            }
                          />
                          <ListItemSecondaryAction>
                            <IconButton
                              edge="end"
                              onClick={() => handleEditConfig(config)}
                            >
                              <Edit />
                            </IconButton>
                          </ListItemSecondaryAction>
                        </ListItem>
                      ))}
                      {getConfigsByCategory('log').concat(getConfigsByCategory('audit')).length === 0 && (
                        <ListItem>
                          <ListItemText primary="No logging configurations found" />
                        </ListItem>
                      )}
                    </List>
                  </CardContent>
                </Card>
              </Grid>

              {/* Performance Settings */}
              <Grid size={{ xs: 12, md: 6 }}>
                <Card>
                  <CardHeader title="Performance Tuning" />
                  <CardContent>
                    <List>
                      {getConfigsByCategory('performance').concat(getConfigsByCategory('cache')).map((config) => (
                        <ListItem key={config.id}>
                          <Box sx={{ mr: 2 }}>
                            <TuneOutlined />
                          </Box>
                          <ListItemText
                            primary={formatConfigKey(config.key)}
                            secondary={
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  {config.description}
                                </Typography>
                                <Chip
                                  label={config.value}
                                  size="small"
                                  sx={{ mt: 1 }}
                                  color="warning"
                                />
                              </Box>
                            }
                          />
                          <ListItemSecondaryAction>
                            <IconButton
                              edge="end"
                              onClick={() => handleEditConfig(config)}
                            >
                              <Edit />
                            </IconButton>
                          </ListItemSecondaryAction>
                        </ListItem>
                      ))}
                      {getConfigsByCategory('performance').concat(getConfigsByCategory('cache')).length === 0 && (
                        <Alert severity="info">
                          No performance configurations found. Default optimizations are active.
                        </Alert>
                      )}
                    </List>
                  </CardContent>
                </Card>
              </Grid>

              {/* Diagnostic Tools */}
              <Grid size={{ xs: 12 }}>
                <Card>
                  <CardHeader title="Diagnostic & Troubleshooting Tools" />
                  <CardContent>
                    <Alert severity="warning" icon={<Warning />} sx={{ mb: 2 }}>
                      Diagnostic tools and configuration export/import functionality will be implemented in future updates.
                    </Alert>
                    <Typography variant="body2" color="text.secondary">
                      These advanced features will include system health checks, configuration backup/restore, 
                      and diagnostic report generation.
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>
          </Box>
        </TabPanel>
      </Paper>

      {/* Edit Configuration Dialog */}
      <Dialog
        open={editDialogOpen}
        onClose={() => setEditDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Edit Configuration</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <TextField
              fullWidth
              label="Configuration Key"
              value={configForm.key}
              disabled
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="Description"
              value={configForm.description}
              disabled
              multiline
              rows={2}
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="Value"
              value={configForm.value}
              onChange={(e) => setConfigForm(prev => ({ ...prev, value: e.target.value }))}
              placeholder="Enter configuration value"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditDialogOpen(false)} startIcon={<Cancel />}>
            Cancel
          </Button>
          <Button
            onClick={() => void handleSaveConfig()}
            variant="contained"
            startIcon={<Save />}
            disabled={loading}
          >
            Save Changes
          </Button>
        </DialogActions>
      </Dialog>

      {/* Password Change Dialog */}
      <Dialog
        open={passwordDialogOpen}
        onClose={() => setPasswordDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Change Password</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <TextField
              fullWidth
              label="Current Password"
              type={showPasswords['old'] ? 'text' : 'password'}
              value={passwordForm.oldPassword}
              onChange={(e) => setPasswordForm(prev => ({ ...prev, oldPassword: e.target.value }))}
              sx={{ mb: 2 }}
              InputProps={{
                endAdornment: (
                  <IconButton
                    aria-label="toggle password visibility"
                    onClick={() => togglePasswordVisibility('old')}
                    edge="end"
                  >
                    {showPasswords['old'] ? <VisibilityOff /> : <Visibility />}
                  </IconButton>
                ),
              }}
            />
            <TextField
              fullWidth
              label="New Password"
              type={showPasswords['new'] ? 'text' : 'password'}
              value={passwordForm.newPassword}
              onChange={(e) => setPasswordForm(prev => ({ ...prev, newPassword: e.target.value }))}
              sx={{ mb: 2 }}
              InputProps={{
                endAdornment: (
                  <IconButton
                    aria-label="toggle password visibility"
                    onClick={() => togglePasswordVisibility('new')}
                    edge="end"
                  >
                    {showPasswords['new'] ? <VisibilityOff /> : <Visibility />}
                  </IconButton>
                ),
              }}
            />
            <TextField
              fullWidth
              label="Confirm New Password"
              type={showPasswords['confirm'] ? 'text' : 'password'}
              value={passwordForm.confirmPassword}
              onChange={(e) => setPasswordForm(prev => ({ ...prev, confirmPassword: e.target.value }))}
              InputProps={{
                endAdornment: (
                  <IconButton
                    aria-label="toggle password visibility"
                    onClick={() => togglePasswordVisibility('confirm')}
                    edge="end"
                  >
                    {showPasswords['confirm'] ? <VisibilityOff /> : <Visibility />}
                  </IconButton>
                ),
              }}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPasswordDialogOpen(false)} startIcon={<Cancel />}>
            Cancel
          </Button>
          <Button
            onClick={() => void handlePasswordChange()}
            variant="contained"
            startIcon={<Security />}
            disabled={loading || !passwordForm.oldPassword || !passwordForm.newPassword || !passwordForm.confirmPassword}
          >
            Change Password
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

export default ConfigPage; 