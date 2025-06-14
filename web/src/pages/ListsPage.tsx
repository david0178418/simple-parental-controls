import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  IconButton,
  Chip,
  Alert,
  CircularProgress,
  Fab,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  FormControlLabel,
  Switch,
  Tabs,
  Tab,
  Card,
  CardContent,
  Tooltip,
  Grid,
} from '@mui/material';
import {
  Add,
  Edit,
  Delete,
  Visibility,
  VisibilityOff,
  Schedule,
  Timer,
  List as ListIcon,
} from '@mui/icons-material';
import { apiClient } from '../services/api';
import type {
  List,
  ListEntry,
  CreateListRequest,
  CreateListEntryRequest,
  ListType,
  EntryType,
  PatternType,
} from '../types/api';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel({ children, value, index }: TabPanelProps) {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
    </div>
  );
}

function ListsPage() {
  const [tabValue, setTabValue] = useState(0);
  const [lists, setLists] = useState<List[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  // List Management State
  const [listDialogOpen, setListDialogOpen] = useState(false);
  const [editingList, setEditingList] = useState<List | null>(null);
  const [listForm, setListForm] = useState<CreateListRequest>({
    name: '',
    type: 'whitelist',
    description: '',
    enabled: true,
  });

  // List Entry Management State
  const [selectedList, setSelectedList] = useState<List | null>(null);
  const [listEntries, setListEntries] = useState<ListEntry[]>([]);
  const [entryDialogOpen, setEntryDialogOpen] = useState(false);
  const [editingEntry, setEditingEntry] = useState<ListEntry | null>(null);
  const [entryForm, setEntryForm] = useState<CreateListEntryRequest>({
    list_id: 0,
    entry_type: 'executable',
    pattern: '',
    pattern_type: 'exact',
    description: '',
    enabled: true,
  });

  useEffect(() => {
    loadLists();
  }, []);

  const loadLists = async (): Promise<void> => {
    try {
      setLoading(true);
      const data = await apiClient.getLists();
      setLists(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load lists');
    } finally {
      setLoading(false);
    }
  };

  const loadListEntries = async (listId: number): Promise<void> => {
    try {
      const entries = await apiClient.getListEntries(listId);
      setListEntries(entries);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load list entries');
    }
  };

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number): void => {
    setTabValue(newValue);
  };

  // List Management Functions
  const handleCreateList = (): void => {
    setEditingList(null);
    setListForm({
      name: '',
      type: 'whitelist',
      description: '',
      enabled: true,
    });
    setListDialogOpen(true);
  };

  const handleEditList = (list: List): void => {
    setEditingList(list);
    setListForm({
      name: list.name,
      type: list.type,
      description: list.description,
      enabled: list.enabled,
    });
    setListDialogOpen(true);
  };

  const handleSaveList = async (): Promise<void> => {
    try {
      if (editingList) {
        await apiClient.updateList({ ...listForm, id: editingList.id });
      } else {
        await apiClient.createList(listForm);
      }
      setListDialogOpen(false);
      await loadLists();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save list');
    }
  };

  const handleDeleteList = async (list: List): Promise<void> => {
    if (window.confirm(`Are you sure you want to delete "${list.name}"?`)) {
      try {
        await apiClient.deleteList(list.id);
        await loadLists();
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to delete list');
      }
    }
  };

  const handleToggleList = async (list: List): Promise<void> => {
    try {
      await apiClient.updateList({
        ...list,
        enabled: !list.enabled,
      });
      await loadLists();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update list');
    }
  };

  // List Entry Management Functions
  const handleViewListEntries = async (list: List): Promise<void> => {
    setSelectedList(list);
    await loadListEntries(list.id);
    setTabValue(1);
  };

  const handleCreateEntry = (): void => {
    if (!selectedList) return;
    setEditingEntry(null);
    setEntryForm({
      list_id: selectedList.id,
      entry_type: 'executable',
      pattern: '',
      pattern_type: 'exact',
      description: '',
      enabled: true,
    });
    setEntryDialogOpen(true);
  };

  const handleEditEntry = (entry: ListEntry): void => {
    setEditingEntry(entry);
    setEntryForm({
      list_id: entry.list_id,
      entry_type: entry.entry_type,
      pattern: entry.pattern,
      pattern_type: entry.pattern_type,
      description: entry.description,
      enabled: entry.enabled,
    });
    setEntryDialogOpen(true);
  };

  const handleSaveEntry = async (): Promise<void> => {
    try {
      if (editingEntry) {
        await apiClient.updateListEntry({ ...entryForm, id: editingEntry.id });
      } else {
        await apiClient.createListEntry(entryForm);
      }
      setEntryDialogOpen(false);
      if (selectedList) {
        await loadListEntries(selectedList.id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save entry');
    }
  };

  const handleDeleteEntry = async (entry: ListEntry): Promise<void> => {
    if (window.confirm(`Are you sure you want to delete this entry?`)) {
      try {
        await apiClient.deleteListEntry(entry.id);
        if (selectedList) {
          await loadListEntries(selectedList.id);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to delete entry');
      }
    }
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Lists & Rules Management
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper sx={{ width: '100%', mb: 2 }}>
        <Tabs
          value={tabValue}
          onChange={handleTabChange}
          indicatorColor="primary"
          textColor="primary"
        >
          <Tab icon={<ListIcon />} label="Lists" />
          <Tab 
            icon={<Edit />} 
            label={selectedList ? `Entries (${selectedList.name})` : 'Entries'} 
            disabled={!selectedList}
          />
          <Tab icon={<Schedule />} label="Time Rules" />
          <Tab icon={<Timer />} label="Quota Rules" />
        </Tabs>

        <TabPanel value={tabValue} index={0}>
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Type</TableCell>
                  <TableCell>Description</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Created</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {lists.map((list) => (
                  <TableRow key={list.id}>
                    <TableCell>
                      <Typography variant="subtitle2">{list.name}</Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={list.type}
                        color={list.type === 'whitelist' ? 'success' : 'error'}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>{list.description}</TableCell>
                    <TableCell>
                      <Chip
                        label={list.enabled ? 'Active' : 'Inactive'}
                        color={list.enabled ? 'success' : 'default'}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      {new Date(list.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell align="right">
                      <Tooltip title="View Entries">
                        <IconButton
                          size="small"
                          onClick={() => handleViewListEntries(list)}
                        >
                          <ListIcon />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title={list.enabled ? 'Disable' : 'Enable'}>
                        <IconButton
                          size="small"
                          onClick={() => handleToggleList(list)}
                        >
                          {list.enabled ? <Visibility /> : <VisibilityOff />}
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Edit">
                        <IconButton
                          size="small"
                          onClick={() => handleEditList(list)}
                        >
                          <Edit />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete">
                        <IconButton
                          size="small"
                          color="error"
                          onClick={() => handleDeleteList(list)}
                        >
                          <Delete />
                        </IconButton>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </TabPanel>

        <TabPanel value={tabValue} index={1}>
          {selectedList && (
            <>
              <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
                <Typography variant="h6">
                  Entries for {selectedList.name}
                </Typography>
                <Button
                  variant="contained"
                  startIcon={<Add />}
                  onClick={handleCreateEntry}
                >
                  Add Entry
                </Button>
              </Box>
              
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Type</TableCell>
                      <TableCell>Pattern</TableCell>
                      <TableCell>Pattern Type</TableCell>
                      <TableCell>Description</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell align="right">Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {listEntries.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell>
                          <Chip
                            label={entry.entry_type}
                            color={entry.entry_type === 'executable' ? 'primary' : 'secondary'}
                            size="small"
                          />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" fontFamily="monospace">
                            {entry.pattern}
                          </Typography>
                        </TableCell>
                        <TableCell>{entry.pattern_type}</TableCell>
                        <TableCell>{entry.description}</TableCell>
                        <TableCell>
                          <Chip
                            label={entry.enabled ? 'Active' : 'Inactive'}
                            color={entry.enabled ? 'success' : 'default'}
                            size="small"
                          />
                        </TableCell>
                        <TableCell align="right">
                          <Tooltip title="Edit">
                            <IconButton
                              size="small"
                              onClick={() => handleEditEntry(entry)}
                            >
                              <Edit />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Delete">
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => handleDeleteEntry(entry)}
                            >
                              <Delete />
                            </IconButton>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}
        </TabPanel>

        <TabPanel value={tabValue} index={2}>
          <Box>
            <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
              <Typography variant="h6">Time Rules</Typography>
              <Button
                variant="contained"
                startIcon={<Add />}
                onClick={() => setError('Time rules functionality coming soon!')}
                disabled={lists.length === 0}
              >
                Add Time Rule
              </Button>
            </Box>

            {lists.length === 0 && (
              <Alert severity="info" sx={{ mb: 2 }}>
                Create at least one list before adding time rules.
              </Alert>
            )}

            <Paper sx={{ p: 3, textAlign: 'center' }}>
              <Schedule sx={{ fontSize: 64, color: 'text.secondary', mb: 2 }} />
              <Typography variant="h6" gutterBottom>
                Time Rules Management
              </Typography>
              <Typography color="text.secondary" paragraph>
                Configure time-based rules to control when applications and websites are allowed or blocked.
                Create schedules for different days of the week with specific time windows.
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Full time rules functionality will be available in the next update.
              </Typography>
            </Paper>
          </Box>
        </TabPanel>

        <TabPanel value={tabValue} index={3}>
          <Box>
            <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
              <Typography variant="h6">Quota Rules</Typography>
              <Button
                variant="contained"
                startIcon={<Add />}
                onClick={() => setError('Quota rules functionality coming soon!')}
                disabled={lists.length === 0}
              >
                Add Quota Rule
              </Button>
            </Box>

            {lists.length === 0 && (
              <Alert severity="info" sx={{ mb: 2 }}>
                Create at least one list before adding quota rules.
              </Alert>
            )}

            <Grid container spacing={3} sx={{ mb: 3 }}>
              <Grid size={{ xs: 12, md: 4 }}>
                <Card>
                  <CardContent>
                    <Box display="flex" alignItems="center" mb={2}>
                      <Timer color="primary" sx={{ mr: 1 }} />
                      <Typography variant="h6">Daily Quotas</Typography>
                    </Box>
                    <Typography variant="h4" color="primary" gutterBottom>
                      0
                    </Typography>
                    <Typography color="text.secondary">
                      Rules configured for daily time limits
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              
              <Grid size={{ xs: 12, md: 4 }}>
                <Card>
                  <CardContent>
                    <Box display="flex" alignItems="center" mb={2}>
                      <Timer color="secondary" sx={{ mr: 1 }} />
                      <Typography variant="h6">Weekly Quotas</Typography>
                    </Box>
                    <Typography variant="h4" color="secondary" gutterBottom>
                      0
                    </Typography>
                    <Typography color="text.secondary">
                      Rules configured for weekly time limits
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              
              <Grid size={{ xs: 12, md: 4 }}>
                <Card>
                  <CardContent>
                    <Box display="flex" alignItems="center" mb={2}>
                      <Timer color="warning" sx={{ mr: 1 }} />
                      <Typography variant="h6">Monthly Quotas</Typography>
                    </Box>
                    <Typography variant="h4" color="warning" gutterBottom>
                      0
                    </Typography>
                    <Typography color="text.secondary">
                      Rules configured for monthly time limits
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>

            <Paper sx={{ p: 3, textAlign: 'center' }}>
              <Timer sx={{ fontSize: 64, color: 'text.secondary', mb: 2 }} />
              <Typography variant="h6" gutterBottom>
                Quota Management System
              </Typography>
              <Typography color="text.secondary" paragraph>
                Set up time-based quotas to limit daily, weekly, or monthly usage of specific applications 
                and websites. Monitor usage patterns and receive alerts when limits are approaching.
              </Typography>
              
              <Box sx={{ mt: 3 }}>
                <Typography variant="subtitle2" gutterBottom>
                  Planned Features:
                </Typography>
                <Grid container spacing={2} sx={{ mt: 1 }}>
                  <Grid size={{ xs: 12, md: 4 }}>
                    <Box sx={{ p: 2, border: '1px dashed', borderColor: 'divider', borderRadius: 1 }}>
                      <Typography variant="body2" fontWeight="medium">
                        Flexible Time Limits
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Daily, weekly, and monthly quotas
                      </Typography>
                    </Box>
                  </Grid>
                  <Grid size={{ xs: 12, md: 4 }}>
                    <Box sx={{ p: 2, border: '1px dashed', borderColor: 'divider', borderRadius: 1 }}>
                      <Typography variant="body2" fontWeight="medium">
                        Usage Tracking
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Real-time usage monitoring and alerts
                      </Typography>
                    </Box>
                  </Grid>
                  <Grid size={{ xs: 12, md: 4 }}>
                    <Box sx={{ p: 2, border: '1px dashed', borderColor: 'divider', borderRadius: 1 }}>
                      <Typography variant="body2" fontWeight="medium">
                        Progress Visualization
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Charts and progress indicators
                      </Typography>
                    </Box>
                  </Grid>
                </Grid>
              </Box>
              
              <Typography variant="body2" color="text.secondary" sx={{ mt: 3 }}>
                Full quota rules functionality will be available in the next update.
              </Typography>
            </Paper>
          </Box>
        </TabPanel>
      </Paper>

      {/* Floating Action Button for Lists Tab */}
      {tabValue === 0 && (
        <Fab
          color="primary"
          aria-label="add list"
          sx={{ position: 'fixed', bottom: 16, right: 16 }}
          onClick={handleCreateList}
        >
          <Add />
        </Fab>
      )}

      {/* List Create/Edit Dialog */}
      <Dialog open={listDialogOpen} onClose={() => setListDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>
          {editingList ? 'Edit List' : 'Create New List'}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="Name"
            fullWidth
            variant="outlined"
            value={listForm.name}
            onChange={(e) => setListForm({ ...listForm, name: e.target.value })}
            sx={{ mb: 2 }}
          />
          <FormControl fullWidth sx={{ mb: 2 }}>
            <InputLabel>Type</InputLabel>
            <Select
              value={listForm.type}
              label="Type"
              onChange={(e) => setListForm({ ...listForm, type: e.target.value as ListType })}
            >
              <MenuItem value="whitelist">Whitelist (Allow)</MenuItem>
              <MenuItem value="blacklist">Blacklist (Block)</MenuItem>
            </Select>
          </FormControl>
          <TextField
            margin="dense"
            label="Description"
            fullWidth
            multiline
            rows={3}
            variant="outlined"
            value={listForm.description}
            onChange={(e) => setListForm({ ...listForm, description: e.target.value })}
            sx={{ mb: 2 }}
          />
          <FormControlLabel
            control={
              <Switch
                checked={listForm.enabled}
                onChange={(e) => setListForm({ ...listForm, enabled: e.target.checked })}
              />
            }
            label="Enabled"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setListDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleSaveList} variant="contained">
            {editingList ? 'Update' : 'Create'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* List Entry Create/Edit Dialog */}
      <Dialog open={entryDialogOpen} onClose={() => setEntryDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>
          {editingEntry ? 'Edit Entry' : 'Add New Entry'}
        </DialogTitle>
        <DialogContent>
          <FormControl fullWidth sx={{ mb: 2, mt: 1 }}>
            <InputLabel>Entry Type</InputLabel>
            <Select
              value={entryForm.entry_type}
              label="Entry Type"
              onChange={(e) => setEntryForm({ ...entryForm, entry_type: e.target.value as EntryType })}
            >
              <MenuItem value="executable">Executable</MenuItem>
              <MenuItem value="url">URL</MenuItem>
            </Select>
          </FormControl>
          <TextField
            margin="dense"
            label="Pattern"
            fullWidth
            variant="outlined"
            value={entryForm.pattern}
            onChange={(e) => setEntryForm({ ...entryForm, pattern: e.target.value })}
            sx={{ mb: 2 }}
            helperText="The pattern to match (e.g., 'chrome.exe' or '*.example.com')"
          />
          <FormControl fullWidth sx={{ mb: 2 }}>
            <InputLabel>Pattern Type</InputLabel>
            <Select
              value={entryForm.pattern_type}
              label="Pattern Type"
              onChange={(e) => setEntryForm({ ...entryForm, pattern_type: e.target.value as PatternType })}
            >
              <MenuItem value="exact">Exact Match</MenuItem>
              <MenuItem value="wildcard">Wildcard</MenuItem>
              <MenuItem value="domain">Domain</MenuItem>
            </Select>
          </FormControl>
          <TextField
            margin="dense"
            label="Description"
            fullWidth
            multiline
            rows={2}
            variant="outlined"
            value={entryForm.description}
            onChange={(e) => setEntryForm({ ...entryForm, description: e.target.value })}
            sx={{ mb: 2 }}
          />
          <FormControlLabel
            control={
              <Switch
                checked={entryForm.enabled}
                onChange={(e) => setEntryForm({ ...entryForm, enabled: e.target.checked })}
              />
            }
            label="Enabled"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEntryDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleSaveEntry} variant="contained">
            {editingEntry ? 'Update' : 'Add'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

export default ListsPage; 