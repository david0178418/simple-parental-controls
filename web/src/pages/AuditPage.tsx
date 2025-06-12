import React, { useState, useEffect, useCallback } from 'react';
import {
  Box,
  Typography,
  Paper,
  Grid,
  Card,
  CardContent,
  TextField,
  Button,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Alert,
  Tooltip,
  Badge,
  Menu,
  ListItem,
  ListItemIcon,
  ListItemText,
  ListItemButton,
  Divider,
  Stack,
  LinearProgress,
} from '@mui/material';
import {
  Security,
  Search,
  FilterList,
  Refresh,
  Download,
  Schedule,
  Info,
  Warning,
  CheckCircle,
  Block,
  Timeline,
  Assessment,
  FileDownload,
  TableChart,
  VisibilityOff,
  Visibility,
} from '@mui/icons-material';
import { DateTimePicker } from '@mui/x-date-pickers/DateTimePicker';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { apiClient, ApiError } from '../services/api';
import { AuditLog, AuditLogFilters, ActionType, TargetType } from '../types/api';

interface FilterFormData extends AuditLogFilters {
  start_time?: string;
  end_time?: string;
  action?: ActionType;
  target_type?: TargetType;
  search?: string;
  limit?: number;
  offset?: number;
}

interface AuditStats {
  total: number;
  today: number;
  blocks: number;
  allows: number;
  recentActivity: boolean;
}

function AuditPage() {
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [filterMenuAnchor, setFilterMenuAnchor] = useState<null | HTMLElement>(null);
  const [exportMenuAnchor, setExportMenuAnchor] = useState<null | HTMLElement>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState<number | null>(null);
  
  const [stats, setStats] = useState<AuditStats>({
    total: 0,
    today: 0,
    blocks: 0,
    allows: 0,
    recentActivity: false,
  });

  const [filters, setFilters] = useState<FilterFormData>({
    limit: 25,
    offset: 0,
  });

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [totalCount, setTotalCount] = useState(0);

  // Load audit logs
  const loadAuditLogs = useCallback(async (showLoading = true): Promise<void> => {
    try {
      if (showLoading) setLoading(true);
      setError(null);
      
      const auditData = await apiClient.getAuditLogs({
        ...filters,
        limit: rowsPerPage,
        offset: page * rowsPerPage,
      });
      
      if (Array.isArray(auditData)) {
        setAuditLogs(auditData);
        setTotalCount(auditData.length); // In a real implementation, this would come from the API
        updateStats(auditData);
      } else {
        setAuditLogs([]);
        setError('Received unexpected data format from server');
      }
    } catch (err) {
      console.error('Failed to load audit logs:', err);
      
      if (err instanceof ApiError && err.status === 404) {
        // Demo data when API is not available
        const demoLogs = generateDemoAuditLogs();
        setAuditLogs(demoLogs);
        setTotalCount(demoLogs.length);
        updateStats(demoLogs);
        setError('Audit API is not yet implemented. Showing demo data for interface preview.');
      } else {
        setAuditLogs([]);
        const errorMsg = err instanceof ApiError ? err.message : 'Failed to load audit logs';
        setError(errorMsg);
      }
    } finally {
      if (showLoading) setLoading(false);
    }
  }, [filters, page, rowsPerPage]);

  // Generate demo audit logs
  const generateDemoAuditLogs = (): AuditLog[] => {
    const events = [
      { event_type: 'access_attempt', target_type: 'executable' as TargetType, target_value: 'games.exe', action: 'block' as ActionType, details: 'Blocked by time rule: School Hours' },
      { event_type: 'access_attempt', target_type: 'url' as TargetType, target_value: 'youtube.com', action: 'allow' as ActionType, details: 'Allowed by whitelist: Educational Sites' },
      { event_type: 'rule_violation', target_type: 'executable' as TargetType, target_value: 'social_app.exe', action: 'block' as ActionType, details: 'Quota exceeded: Daily limit reached' },
      { event_type: 'system_event', target_type: 'url' as TargetType, target_value: 'study.com', action: 'allow' as ActionType, details: 'Educational content permitted' },
      { event_type: 'access_attempt', target_type: 'executable' as TargetType, target_value: 'browser.exe', action: 'allow' as ActionType, details: 'Standard application access' },
      { event_type: 'policy_change', target_type: 'url' as TargetType, target_value: 'config.system', action: 'allow' as ActionType, details: 'Time rule updated by administrator' },
    ];

    return Array.from({ length: 50 }, (_, index) => {
      const event = events[index % events.length];
      if (!event) {
        throw new Error('Event not found'); // This should never happen
      }
      const timestamp = new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000);
      const log: AuditLog = {
        id: index + 1,
        timestamp: timestamp.toISOString(),
        event_type: event.event_type,
        target_type: event.target_type,
        target_value: event.target_value,
        action: event.action,
        details: event.details,
        created_at: timestamp.toISOString(),
      };
      
      // Add optional fields
      if (index % 3 === 0) {
        log.rule_type = 'time_rule';
        log.rule_id = Math.floor(Math.random() * 10) + 1;
      } else if (index % 3 === 1) {
        log.rule_type = 'quota_rule';
        log.rule_id = Math.floor(Math.random() * 10) + 1;
      }
      
      return log;
    });
  };

  // Update statistics
  const updateStats = (logs: AuditLog[]): void => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    
    const todayLogs = logs.filter(log => new Date(log.timestamp) >= today);
    const blocks = logs.filter(log => log.action === 'block').length;
    const allows = logs.filter(log => log.action === 'allow').length;
    const recentActivity = logs.some(log => 
      new Date(log.timestamp) > new Date(Date.now() - 5 * 60 * 1000) // Last 5 minutes
    );

    setStats({
      total: logs.length,
      today: todayLogs.length,
      blocks,
      allows,
      recentActivity,
    });
  };

  useEffect(() => {
    void loadAuditLogs();
  }, [loadAuditLogs]);

  // Auto-refresh functionality
  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(() => {
        void loadAuditLogs(false);
      }, 30000); // Refresh every 30 seconds
      setRefreshInterval(interval);
    } else if (refreshInterval) {
      clearInterval(refreshInterval);
      setRefreshInterval(null);
    }

    return () => {
      if (refreshInterval) {
        clearInterval(refreshInterval);
      }
    };
  }, [autoRefresh, loadAuditLogs, refreshInterval]);

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

  const handleFilterChange = (key: keyof FilterFormData, value: string | number | undefined): void => {
    setFilters(prev => ({ ...prev, [key]: value }));
    setPage(0); // Reset to first page when filtering
  };

  const handlePageChange = (_: unknown, newPage: number): void => {
    setPage(newPage);
  };

  const handleRowsPerPageChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  const handleLogClick = (log: AuditLog): void => {
    setSelectedLog(log);
    setDetailDialogOpen(true);
  };

  const handleExport = async (format: 'csv' | 'json'): Promise<void> => {
    try {
      setExportMenuAnchor(null);
      
      if (format === 'csv') {
        const csvContent = [
          'ID,Timestamp,Event Type,Target Type,Target Value,Action,Rule Type,Rule ID,Details',
          ...auditLogs.map(log => 
            `${log.id},"${log.timestamp}","${log.event_type}","${log.target_type}","${log.target_value}","${log.action}","${log.rule_type || ''}","${log.rule_id || ''}","${log.details}"`
          )
        ].join('\n');
        
        const blob = new Blob([csvContent], { type: 'text/csv' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `audit-logs-${new Date().toISOString().split('T')[0]}.csv`;
        a.click();
        URL.revokeObjectURL(url);
      } else {
        const jsonContent = JSON.stringify(auditLogs, null, 2);
        const blob = new Blob([jsonContent], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `audit-logs-${new Date().toISOString().split('T')[0]}.json`;
        a.click();
        URL.revokeObjectURL(url);
      }
      
      setSuccess(`Audit logs exported successfully as ${format.toUpperCase()}`);
    } catch (err) {
      setError('Failed to export audit logs');
    }
  };

  const clearFilters = (): void => {
    setFilters({ limit: 25, offset: 0 });
    setPage(0);
    setFilterMenuAnchor(null);
  };

  const formatTimestamp = (timestamp: string): string => {
    return new Date(timestamp).toLocaleString();
  };

  const getActionIcon = (action: ActionType): React.ReactNode => {
    switch (action) {
      case 'allow':
        return <CheckCircle color="success" fontSize="small" />;
      case 'block':
        return <Block color="error" fontSize="small" />;
      default:
        return <Info color="info" fontSize="small" />;
    }
  };

  const getActionColor = (action: ActionType): 'success' | 'error' | 'default' => {
    switch (action) {
      case 'allow':
        return 'success';
      case 'block':
        return 'error';
      default:
        return 'default';
    }
  };

  const getEventTypeIcon = (eventType: string): React.ReactNode => {
    switch (eventType) {
      case 'access_attempt':
        return <Security fontSize="small" />;
      case 'rule_violation':
        return <Warning fontSize="small" />;
      case 'system_event':
        return <Info fontSize="small" />;
      case 'policy_change':
        return <Schedule fontSize="small" />;
      default:
        return <Timeline fontSize="small" />;
    }
  };

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', mb: 3 }}>
          <Security sx={{ mr: 2, fontSize: 32 }} />
          <Typography variant="h4" gutterBottom sx={{ mb: 0, flexGrow: 1 }}>
            Audit Log Viewer
          </Typography>
          
          {/* Auto-refresh indicator */}
          {autoRefresh && (
            <Badge 
              color="success" 
              variant="dot" 
              sx={{ mr: 2 }}
            >
              <Chip 
                icon={<Refresh />} 
                label="Auto-refresh" 
                size="small" 
                color="success"
                onClick={() => setAutoRefresh(false)}
              />
            </Badge>
          )}
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

        {/* Statistics Cards */}
        <Grid container spacing={3} sx={{ mb: 3 }}>
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <Assessment color="primary" sx={{ mr: 2 }} />
                  <Box>
                    <Typography variant="h6">{stats.total}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Total Events
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
          
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <Schedule color="info" sx={{ mr: 2 }} />
                  <Box>
                    <Typography variant="h6">{stats.today}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Today's Events
                    </Typography>
                  </Box>
                  {stats.recentActivity && (
                    <Badge color="success" variant="dot" sx={{ ml: 'auto' }} />
                  )}
                </Box>
              </CardContent>
            </Card>
          </Grid>
          
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <Block color="error" sx={{ mr: 2 }} />
                  <Box>
                    <Typography variant="h6">{stats.blocks}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Blocked Actions
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
          
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <CheckCircle color="success" sx={{ mr: 2 }} />
                  <Box>
                    <Typography variant="h6">{stats.allows}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Allowed Actions
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Filters and Controls */}
        <Paper sx={{ p: 3, mb: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
            <Typography variant="h6" sx={{ flexGrow: 1 }}>
              Filters & Search
            </Typography>
            
            <Stack direction="row" spacing={1}>
              <Button
                startIcon={<FilterList />}
                onClick={(e) => setFilterMenuAnchor(e.currentTarget)}
                variant="outlined"
                size="small"
              >
                Advanced
              </Button>
              
              <Button
                startIcon={<Download />}
                onClick={(e) => setExportMenuAnchor(e.currentTarget)}
                variant="outlined"
                size="small"
              >
                Export
              </Button>
              
              <Button
                startIcon={autoRefresh ? <VisibilityOff /> : <Visibility />}
                onClick={() => setAutoRefresh(!autoRefresh)}
                variant={autoRefresh ? "contained" : "outlined"}
                size="small"
                color={autoRefresh ? "success" : "primary"}
              >
                {autoRefresh ? 'Stop' : 'Auto'} Refresh
              </Button>
              
              <Button
                startIcon={<Refresh />}
                onClick={() => void loadAuditLogs()}
                disabled={loading}
                variant="outlined"
                size="small"
              >
                Refresh
              </Button>
            </Stack>
          </Box>

          <Grid container spacing={2} alignItems="center">
            <Grid size={{ xs: 12, md: 4 }}>
              <TextField
                fullWidth
                label="Search logs"
                placeholder="Search events, targets, details..."
                value={filters.search || ''}
                onChange={(e) => handleFilterChange('search', e.target.value)}
                InputProps={{
                  startAdornment: <Search sx={{ mr: 1, color: 'text.secondary' }} />,
                }}
                size="small"
              />
            </Grid>
            
            <Grid size={{ xs: 12, sm: 4, md: 2 }}>
              <FormControl fullWidth size="small">
                <InputLabel>Action</InputLabel>
                <Select
                  value={filters.action || ''}
                  label="Action"
                  onChange={(e) => handleFilterChange('action', e.target.value as ActionType)}
                >
                  <MenuItem value="">All Actions</MenuItem>
                  <MenuItem value="allow">Allow</MenuItem>
                  <MenuItem value="block">Block</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            
            <Grid size={{ xs: 12, sm: 4, md: 2 }}>
              <FormControl fullWidth size="small">
                <InputLabel>Target Type</InputLabel>
                <Select
                  value={filters.target_type || ''}
                  label="Target Type"
                  onChange={(e) => handleFilterChange('target_type', e.target.value as TargetType)}
                >
                  <MenuItem value="">All Types</MenuItem>
                  <MenuItem value="executable">Executable</MenuItem>
                  <MenuItem value="url">URL</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            
            <Grid size={{ xs: 12, sm: 4, md: 2 }}>
              <DateTimePicker
                label="Start Time"
                value={filters.start_time ? new Date(filters.start_time) : null}
                onChange={(date) => handleFilterChange('start_time', date?.toISOString())}
                slotProps={{ 
                  textField: { size: 'small', fullWidth: true } 
                }}
              />
            </Grid>
            
            <Grid size={{ xs: 12, sm: 4, md: 2 }}>
              <DateTimePicker
                label="End Time"
                value={filters.end_time ? new Date(filters.end_time) : null}
                onChange={(date) => handleFilterChange('end_time', date?.toISOString())}
                slotProps={{ 
                  textField: { size: 'small', fullWidth: true } 
                }}
              />
            </Grid>
          </Grid>
        </Paper>

        {/* Audit Logs Table */}
        <Paper sx={{ width: '100%' }}>
          <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="h6">
              Audit Events
              {!loading && (
                <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                  ({auditLogs.length} events)
                </Typography>
              )}
            </Typography>
          </Box>

          {loading && <LinearProgress />}

          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Timestamp</TableCell>
                  <TableCell>Event Type</TableCell>
                  <TableCell>Target</TableCell>
                  <TableCell>Action</TableCell>
                  <TableCell>Rule</TableCell>
                  <TableCell>Details</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {auditLogs.length === 0 && !loading ? (
                  <TableRow>
                    <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                      <Typography variant="body2" color="text.secondary">
                        No audit logs found. Try adjusting your filters or check back later.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  auditLogs.map((log) => (
                    <TableRow 
                      key={log.id} 
                      hover 
                      sx={{ cursor: 'pointer' }}
                      onClick={() => handleLogClick(log)}
                    >
                      <TableCell>
                        <Typography variant="body2">
                          {formatTimestamp(log.timestamp)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                          {getEventTypeIcon(log.event_type)}
                          <Typography variant="body2" sx={{ ml: 1 }}>
                            {log.event_type.replace('_', ' ')}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" fontWeight="medium">
                            {log.target_value}
                          </Typography>
                          <Chip 
                            label={log.target_type} 
                            size="small" 
                            variant="outlined"
                            sx={{ mt: 0.5 }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                          {getActionIcon(log.action)}
                          <Chip 
                            label={log.action}
                            color={getActionColor(log.action)}
                            size="small"
                            sx={{ ml: 1 }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        {log.rule_type && (
                          <Typography variant="body2">
                            {log.rule_type} #{log.rule_id}
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography 
                          variant="body2" 
                          sx={{ 
                            maxWidth: 200, 
                            overflow: 'hidden', 
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap'
                          }}
                        >
                          {log.details}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <Tooltip title="View details">
                          <IconButton size="small">
                            <Info />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>

          <TablePagination
            rowsPerPageOptions={[10, 25, 50, 100]}
            component="div"
            count={totalCount}
            rowsPerPage={rowsPerPage}
            page={page}
            onPageChange={handlePageChange}
            onRowsPerPageChange={handleRowsPerPageChange}
          />
        </Paper>

        {/* Filter Menu */}
        <Menu
          anchorEl={filterMenuAnchor}
          open={Boolean(filterMenuAnchor)}
          onClose={() => setFilterMenuAnchor(null)}
        >
          <ListItem>
            <ListItemText primary="Advanced Filters" secondary="More filtering options coming soon" />
          </ListItem>
          <Divider />
          <ListItemButton onClick={clearFilters}>
            <ListItemIcon>
              <FilterList />
            </ListItemIcon>
            <ListItemText primary="Clear All Filters" />
          </ListItemButton>
        </Menu>

        {/* Export Menu */}
        <Menu
          anchorEl={exportMenuAnchor}
          open={Boolean(exportMenuAnchor)}
          onClose={() => setExportMenuAnchor(null)}
        >
          <ListItemButton onClick={() => void handleExport('csv')}>
            <ListItemIcon>
              <TableChart />
            </ListItemIcon>
            <ListItemText primary="Export as CSV" />
          </ListItemButton>
          <ListItemButton onClick={() => void handleExport('json')}>
            <ListItemIcon>
              <FileDownload />
            </ListItemIcon>
            <ListItemText primary="Export as JSON" />
          </ListItemButton>
        </Menu>

        {/* Log Detail Dialog */}
        <Dialog 
          open={detailDialogOpen} 
          onClose={() => setDetailDialogOpen(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              {selectedLog && getEventTypeIcon(selectedLog.event_type)}
              <Typography variant="h6" sx={{ ml: 1 }}>
                Audit Log Details
              </Typography>
            </Box>
          </DialogTitle>
          <DialogContent>
            {selectedLog && (
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <Typography variant="subtitle2" gutterBottom>Event Information</Typography>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">ID:</Typography>
                    <Typography variant="body1">{selectedLog.id}</Typography>
                  </Box>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">Timestamp:</Typography>
                    <Typography variant="body1">{formatTimestamp(selectedLog.timestamp)}</Typography>
                  </Box>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">Event Type:</Typography>
                    <Typography variant="body1">{selectedLog.event_type.replace('_', ' ')}</Typography>
                  </Box>
                </Grid>
                
                <Grid size={{ xs: 12, sm: 6 }}>
                  <Typography variant="subtitle2" gutterBottom>Target & Action</Typography>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">Target Type:</Typography>
                    <Chip label={selectedLog.target_type} size="small" />
                  </Box>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">Target Value:</Typography>
                    <Typography variant="body1">{selectedLog.target_value}</Typography>
                  </Box>
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary">Action:</Typography>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      {getActionIcon(selectedLog.action)}
                      <Chip 
                        label={selectedLog.action}
                        color={getActionColor(selectedLog.action)}
                        size="small"
                        sx={{ ml: 1 }}
                      />
                    </Box>
                  </Box>
                </Grid>
                
                {selectedLog.rule_type && (
                  <Grid size={{ xs: 12 }}>
                    <Typography variant="subtitle2" gutterBottom>Rule Information</Typography>
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="body2" color="text.secondary">Rule Type:</Typography>
                      <Typography variant="body1">{selectedLog.rule_type}</Typography>
                    </Box>
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="body2" color="text.secondary">Rule ID:</Typography>
                      <Typography variant="body1">{selectedLog.rule_id}</Typography>
                    </Box>
                  </Grid>
                )}
                
                <Grid size={{ xs: 12 }}>
                  <Typography variant="subtitle2" gutterBottom>Details</Typography>
                  <Paper sx={{ p: 2, bgcolor: 'grey.50' }}>
                    <Typography variant="body1">{selectedLog.details}</Typography>
                  </Paper>
                </Grid>
              </Grid>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
          </DialogActions>
        </Dialog>
      </Box>
    </LocalizationProvider>
  );
}

export default AuditPage; 