import { Box, Typography, Button } from '@mui/material';
import ArrowForwardIcon from '@mui/icons-material/ArrowForward';
import { useUploadStore } from '../../store/uploadStore';

export const TrainingProgress = ({ onBegin }) => {
    const { selectedMethods, fileTree, methodStatus } = useUploadStore();

    if (selectedMethods.size === 0) return null;

    const filesCount = new Set(Array.from(selectedMethods).map(id => id.split('::')[0])).size;

    let newSelect = 0;
    let retrainSelect = 0;
    selectedMethods.forEach(id => {
        const stat = methodStatus.get(id) || 'not_trained';
        if (stat === 'trained') retrainSelect++;
        else newSelect++;
    });

    return (
        <Box sx={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            bgcolor: '#fff',
            borderTop: '1px solid #e2e8f0',
            p: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            boxShadow: '0 -2px 8px rgba(0,0,0,0.06)',
            zIndex: 10
        }}>
            <Box>
                <Typography variant="body2" sx={{ fontWeight: 600, color: '#0f172a' }}>
                    {selectedMethods.size} methods selected across {filesCount} files
                </Typography>
                <Typography variant="caption" sx={{ color: '#64748b' }}>
                    Trained: {retrainSelect} · New: {newSelect}
                </Typography>
            </Box>
            <Button
                variant="contained"
                color="primary"
                onClick={onBegin}
                endIcon={<ArrowForwardIcon />}
            >
                Begin Training
            </Button>
        </Box>
    );
};
