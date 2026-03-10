import { Box, Typography, TextField, Button, Chip, CircularProgress } from '@mui/material';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import { useState } from 'react';
import { useUploadStore } from '../../store/uploadStore';

const EXAMPLES = [
    '3 appearing papers',
    '2 failed + 1 appearing, late',
    'PWD exemption',
    'custom fee enabled',
    're-course papers'
];

export const QueryPanel = ({ onRun, isRunning }) => {
    const { activeFile, fileTree, setActiveFile } = useUploadStore();
    const [query, setQuery] = useState('');

    // Get distinct filenames from the parsed tree
    const distinctFiles = Array.from(new Set(Object.values(fileTree).map(f => f.filename)));

    return (
        <Box sx={{ p: 3, pb: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>

            {/* Breadcrumb */}
            <Typography variant="caption" sx={{ color: '#64748b', fontFamily: 'inherit' }}>
                Files &gt; {activeFile || 'No file selected'}
            </Typography>

            {/* File Chips */}
            {distinctFiles.length > 0 && (
                <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                    {distinctFiles.map(fn => (
                        <Chip
                            key={fn}
                            label={fn}
                            onClick={() => setActiveFile(fn)}
                            onDelete={activeFile === fn ? () => setActiveFile(null) : undefined}
                            color={activeFile === fn ? 'primary' : 'default'}
                            variant={activeFile === fn ? 'filled' : 'outlined'}
                            size="small"
                            sx={{ fontFamily: 'inherit', fontWeight: 600 }}
                        />
                    ))}
                </Box>
            )}

            {/* Input */}
            <TextField
                multiline
                minRows={3}
                maxRows={5}
                fullWidth
                placeholder="e.g. student with 3 appearing papers Term 1, submitted on time"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                variant="outlined"
            />

            {/* Examples */}
            <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                {EXAMPLES.map(ex => (
                    <Chip
                        key={ex}
                        label={ex}
                        size="small"
                        onClick={() => setQuery(ex)}
                        sx={{ bgcolor: '#f1f5f9', color: '#475569', '&:hover': { bgcolor: '#e2e8f0' } }}
                    />
                ))}
            </Box>

            {/* Run Button */}
            <Button
                variant="contained"
                color="primary"
                fullWidth
                size="large"
                disabled={!activeFile || !query.trim() || isRunning}
                onClick={() => onRun(query)}
                startIcon={isRunning ? <CircularProgress size={16} color="inherit" /> : <PlayArrowIcon />}
            >
                {isRunning ? 'Running...' : 'Run Pipeline'}
            </Button>
        </Box>
    );
};
