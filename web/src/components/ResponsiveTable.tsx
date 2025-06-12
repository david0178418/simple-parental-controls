import React from 'react';
import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Box,
  Card,
  CardContent,
  Typography,
  Stack,
  useMediaQuery,
  useTheme,
  IconButton,
} from '@mui/material';
import { MoreVert } from '@mui/icons-material';

export interface TableColumn {
  id: string;
  label: string;
  minWidth?: number;
  align?: 'left' | 'right' | 'center';
  format?: (value: unknown, row?: TableRow) => React.ReactNode;
  sortable?: boolean;
  mobileLabel?: string;
  hideOnMobile?: boolean;
}

export interface TableRow {
  id: string | number;
  [key: string]: unknown;
}

interface ResponsiveTableProps {
  columns: TableColumn[];
  rows: TableRow[];
  onRowClick?: (row: TableRow) => void;
  onRowAction?: (row: TableRow) => void;
  emptyMessage?: string;
  stickyHeader?: boolean;
  dense?: boolean;
}

function ResponsiveTable({
  columns,
  rows,
  onRowClick,
  onRowAction,
  emptyMessage = 'No data available',
  stickyHeader = false,
  dense = false,
}: ResponsiveTableProps) {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));

  const visibleColumns = isMobile ? columns.filter(col => !col.hideOnMobile) : columns;

  // Mobile card view
  if (isMobile) {
    return (
      <Box>
        {rows.length === 0 ? (
          <Paper sx={{ p: 3, textAlign: 'center' }}>
            <Typography variant="body2" color="text.secondary">
              {emptyMessage}
            </Typography>
          </Paper>
        ) : (
          <Stack spacing={2}>
            {rows.map((row) => (
              <Card 
                key={row.id} 
                sx={{ 
                  cursor: onRowClick ? 'pointer' : 'default',
                  '&:hover': onRowClick ? {
                    elevation: 4,
                    transform: 'translateY(-1px)',
                    transition: 'all 0.2s ease-in-out',
                  } : {},
                }}
                onClick={() => onRowClick?.(row)}
              >
                <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
                    <Typography variant="subtitle2" component="div" sx={{ fontWeight: 600 }}>
                      {visibleColumns[0] && visibleColumns[0].format ? 
                        visibleColumns[0].format(row[visibleColumns[0].id]) : 
                        String(row[visibleColumns[0]?.id || ''])
                      }
                    </Typography>
                    {onRowAction && (
                      <IconButton
                        size="small"
                        onClick={(e) => {
                          e.stopPropagation();
                          onRowAction(row);
                        }}
                      >
                        <MoreVert />
                      </IconButton>
                    )}
                  </Box>
                  
                  <Stack spacing={1}>
                    {visibleColumns.slice(1).map((column) => {
                      const value = row[column.id];
                      const displayValue = column.format ? column.format(value, row) : String(value);
                      
                      return (
                        <Box key={column.id} sx={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Typography variant="body2" color="text.secondary" sx={{ minWidth: '30%' }}>
                            {column.mobileLabel || column.label}:
                          </Typography>
                          <Box sx={{ 
                            textAlign: 'right', 
                            flex: 1,
                            ml: 1,
                            display: 'flex',
                            justifyContent: 'flex-end',
                            alignItems: 'center',
                          }}>
                            {React.isValidElement(displayValue) ? displayValue : (
                              <Typography variant="body2" component="span">
                                {displayValue}
                              </Typography>
                            )}
                          </Box>
                        </Box>
                      );
                    })}
                  </Stack>
                </CardContent>
              </Card>
            ))}
          </Stack>
        )}
      </Box>
    );
  }

  // Desktop table view
  return (
    <TableContainer component={Paper} sx={{ maxWidth: '100%', overflowX: 'auto' }}>
      <Table stickyHeader={stickyHeader} size={dense ? 'small' : 'medium'}>
        <TableHead>
          <TableRow>
            {columns.map((column) => (
              <TableCell
                key={column.id}
                align={column.align || 'left'}
                style={{ minWidth: column.minWidth }}
                sx={{ fontWeight: 600 }}
              >
                {column.label}
              </TableCell>
            ))}
            {onRowAction && (
              <TableCell align="right" sx={{ fontWeight: 600 }}>
                Actions
              </TableCell>
            )}
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell 
                colSpan={columns.length + (onRowAction ? 1 : 0)} 
                align="center" 
                sx={{ py: 4 }}
              >
                <Typography variant="body2" color="text.secondary">
                  {emptyMessage}
                </Typography>
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow 
                hover 
                key={row.id}
                sx={{ 
                  cursor: onRowClick ? 'pointer' : 'default',
                  '&:last-child td, &:last-child th': { border: 0 }
                }}
                onClick={() => onRowClick?.(row)}
              >
                {columns.map((column) => {
                  const value = row[column.id];
                  const displayValue = column.format ? column.format(value, row) : String(value);
                  
                  return (
                    <TableCell key={column.id} align={column.align || 'left'}>
                      {displayValue}
                    </TableCell>
                  );
                })}
                {onRowAction && (
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation();
                        onRowAction(row);
                      }}
                    >
                      <MoreVert />
                    </IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

export default ResponsiveTable; 