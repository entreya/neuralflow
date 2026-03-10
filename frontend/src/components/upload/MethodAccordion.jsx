import { Box, Typography, Button, Accordion, AccordionSummary, AccordionDetails, Chip, CircularProgress, Tooltip, IconButton } from '@mui/material';
import ArrowRightIcon from '@mui/icons-material/ArrowRight';
import FolderIcon from '@mui/icons-material/Folder';
import CheckIcon from '@mui/icons-material/Check';
import ErrorIcon from '@mui/icons-material/Error';
import RefreshIcon from '@mui/icons-material/Refresh';
import { DataGrid } from '@mui/x-data-grid';
import { useUploadStore } from '../../store/uploadStore';

export const MethodAccordion = () => {
    const { fileTree, selectedMethods, methodStatus, selectAllInFile, deselectAllInFile, toggleMethod, selectAll, setFileSelection } = useUploadStore();

    const handleSelectAll = (filePath, e) => {
        e.stopPropagation();
        selectAllInFile(filePath);
    };

    const handleDeselectAll = (filePath, e) => {
        e.stopPropagation();
        deselectAllInFile(filePath);
    };

    const getStatusChip = (status) => {
        switch (status) {
            case 'trained': return <Chip label="trained" size="small" sx={{ bgcolor: '#22c55e', color: '#fff' }} icon={<CheckIcon sx={{ fontSize: 14, color: '#fff !important' }} />} />;
            case 'processing': return <Chip label="processing" size="small" sx={{ bgcolor: '#f59e0b', color: '#fff' }} icon={<CircularProgress size={10} sx={{ color: '#fff' }} />} />;
            case 'queued': return <Chip label="queued" size="small" variant="outlined" sx={{ color: '#3b82f6', borderColor: '#bfdbfe' }} />;
            case 'failed': return <Chip label="failed" size="small" sx={{ bgcolor: '#ef4444', color: '#fff' }} icon={<ErrorIcon sx={{ fontSize: 14, color: '#fff !important' }} />} />;
            default: return <Chip label="not trained" size="small" variant="outlined" sx={{ color: '#64748b', borderColor: '#cbd5e1' }} />;
        }
    };

    const files = Object.values(fileTree);
    if (files.length === 0) return null;

    return (
        <Box sx={{ mt: 3 }}>
            {/* Global Toolbar */}
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2, px: 1 }}>
                <Typography variant="caption" sx={{ color: '#64748b', fontWeight: 600 }}>
                    {files.length} files · {selectedMethods.size} methods selected
                </Typography>
                <Button size="small" onClick={() => selectAll()} sx={{ color: '#64748b', fontSize: '0.75rem' }}>
                    [☐ Select All Files]
                </Button>
            </Box>

            {files.map(file => {
                const total = file.methods.length;
                const trained = file.methods.filter(m => methodStatus.get(`${file.path}::${m.name}`) === 'trained').length;
                const untrained = total - trained;

                const columns = [
                    {
                        field: 'name',
                        headerName: 'Method',
                        flex: 1,
                        renderCell: (params) => <Typography variant="body2" sx={{ fontFamily: 'inherit', fontWeight: 600 }}>{params.value}()</Typography>
                    },
                    {
                        field: 'visibility',
                        headerName: 'Visibility',
                        width: 100,
                        renderCell: (params) => <Chip label={params.value} size="small" sx={{ height: 20, fontSize: '0.65rem' }} />
                    },
                    {
                        field: 'lines',
                        headerName: 'Lines',
                        width: 70,
                        align: 'right',
                        headerAlign: 'right'
                    },
                    {
                        field: 'status',
                        headerName: 'Status',
                        width: 120,
                        renderCell: (params) => {
                            const id = `${file.path}::${params.row.name}`;
                            return getStatusChip(methodStatus.get(id) || params.value);
                        }
                    },
                    {
                        field: 'actions',
                        headerName: 'Actions',
                        width: 70,
                        sortable: false,
                        renderCell: (params) => {
                            const id = `${file.path}::${params.row.name}`;
                            const stat = methodStatus.get(id) || params.row.status;
                            if (stat === 'trained') {
                                return (
                                    <Tooltip title="Retrain">
                                        <IconButton size="small"><RefreshIcon sx={{ fontSize: 16 }} /></IconButton>
                                    </Tooltip>
                                );
                            }
                            return null;
                        }
                    }
                ];

                const rows = file.methods.map((m, idx) => ({ id: m.name, ...m }));

                return (
                    <Accordion key={file.path} defaultExpanded>
                        <AccordionSummary expandIcon={<ArrowRightIcon />}>
                            <Box sx={{ display: 'flex', width: '100%', alignItems: 'center', pr: 2 }}>
                                <FolderIcon sx={{ color: '#94a3b8', fontSize: 18, mr: 1 }} />
                                <Typography variant="body2" sx={{ fontFamily: 'inherit', flex: 1 }}>{file.path}</Typography>

                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                                    <Typography variant="caption" sx={{ color: '#64748b' }}>
                                        {total} methods | {trained} trained | {untrained} new
                                    </Typography>
                                    <Button size="small" onClick={(e) => handleSelectAll(file.path, e)} sx={{ minWidth: 0, p: 0.5, fontSize: '0.7rem', color: '#64748b' }}>[All]</Button>
                                    <Button size="small" onClick={(e) => handleDeselectAll(file.path, e)} sx={{ minWidth: 0, p: 0.5, fontSize: '0.7rem', color: '#64748b' }}>[None]</Button>
                                </Box>
                            </Box>
                        </AccordionSummary>
                        <AccordionDetails sx={{ p: 0 }}>
                            <DataGrid
                                rows={rows}
                                columns={columns}
                                checkboxSelection
                                disableRowSelectionOnClick
                                hideFooter
                                density="compact"
                                autoHeight
                                rowSelectionModel={rows.filter(r => selectedMethods.has(`${file.path}::${r.name}`)).map(r => r.id)}
                                onRowSelectionModelChange={(newSelectionModel) => {
                                    // newSelectionModel is array of row IDs (method names) now selected
                                    setFileSelection(file.path, newSelectionModel);
                                }}
                                getRowClassName={(params) => {
                                    const id = `${file.path}::${params.row.name}`;
                                    const stat = methodStatus.get(id) || params.row.status;
                                    if (stat === 'trained') return 'trained-row';
                                    if (stat === 'queued') return 'queued-row';
                                    return '';
                                }}
                                sx={{
                                    '& .trained-row': { bgcolor: '#f0fdf4 !important', '& .MuiCheckbox-root': { color: '#22c55e' } },
                                    '& .queued-row': { bgcolor: '#eff6ff !important' },
                                    '& .MuiCheckbox-root': { color: '#0f172a' }
                                }}
                            />
                        </AccordionDetails>
                    </Accordion>
                );
            })}
        </Box>
    );
};
