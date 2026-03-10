import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Typography, Box, Chip } from '@mui/material';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import ArrowForwardIcon from '@mui/icons-material/ArrowForward';
import { useUploadStore } from '../../store/uploadStore';
import { useModelStore } from '../../store/modelStore';

export const TrainingConfirmDialog = ({ open, onClose, onStart }) => {
    const { fileTree, selectedMethods, methodStatus } = useUploadStore();
    const { chatModel } = useModelStore();

    const methodsList = Array.from(selectedMethods).map(id => {
        const [path, name] = id.split('::');
        return { path, name, id };
    });

    const filesCount = new Set(methodsList.map(m => m.path)).size;
    const trainedSelected = methodsList.filter(m => (methodStatus.get(m.id) || 'not_trained') === 'trained').length;
    const newSelected = methodsList.length - trainedSelected;
    const estTimeMin = Math.ceil((newSelected * 30 + trainedSelected * 35) / 60);

    // Group to display nicely
    const grouped = {};
    methodsList.forEach(m => {
        if (!grouped[m.path]) grouped[m.path] = [];
        grouped[m.path].push(m);
    });

    return (
        <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth PaperProps={{ sx: { borderRadius: 3 } }}>
            <DialogTitle sx={{ fontWeight: 700, fontFamily: "inherit" }}>Confirm Training</DialogTitle>
            <DialogContent>
                {/* Summary Card */}
                <Box sx={{ bgcolor: '#f8fafc', p: 2, borderRadius: 2, mb: 3 }}>
                    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
                        <Box>
                            <Typography variant="caption" color="text.secondary">Files</Typography>
                            <Typography variant="body1" fontWeight={600} fontFamily="inherit">{filesCount}</Typography>
                        </Box>
                        <Box>
                            <Typography variant="caption" color="text.secondary">Methods</Typography>
                            <Typography variant="body1" fontWeight={600} fontFamily="inherit">{methodsList.length} total <Typography component="span" variant="caption" color="text.secondary">({newSelected} new · {trainedSelected} retrain)</Typography></Typography>
                        </Box>
                        <Box>
                            <Typography variant="caption" color="text.secondary">Model</Typography>
                            <Typography variant="body1" fontWeight={600} fontFamily="inherit">{chatModel}</Typography>
                        </Box>
                        <Box>
                            <Typography variant="caption" color="text.secondary">Est. time</Typography>
                            <Typography variant="body1" fontWeight={600} fontFamily="inherit">~{estTimeMin} minutes</Typography>
                        </Box>
                    </Box>
                </Box>

                {/* Warning if retraining */}
                {trainedSelected > 0 && (
                    <Box sx={{ bgcolor: '#fffbeb', border: '1px solid #fde68a', p: 2, borderRadius: 2, display: 'flex', gap: 1.5, mb: 3 }}>
                        <WarningAmberIcon sx={{ color: '#d97706', mt: 0.5 }} />
                        <Box>
                            <Typography variant="body2" sx={{ color: '#92400e', fontWeight: 600 }}>{trainedSelected} already-trained methods will be retrained.</Typography>
                            <Typography variant="caption" sx={{ color: '#92400e' }}>Existing QA pairs and verbalizations will be seamlessly replaced.</Typography>
                        </Box>
                    </Box>
                )}

                {/* Scrollable List */}
                <Box sx={{ maxHeight: 200, overflowY: 'auto', border: '1px solid #e2e8f0', borderRadius: 2, p: 1.5 }}>
                    {Object.entries(grouped).map(([path, methods]) => (
                        <Box key={path} sx={{ mb: 2, '&:last-child': { mb: 0 } }}>
                            <Typography variant="caption" sx={{ color: '#64748b', display: 'block', mb: 1, fontFamily: 'inherit' }}>{path}</Typography>
                            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                                {methods.map(m => {
                                    const isRetrain = (methodStatus.get(m.id) || 'not_trained') === 'trained';
                                    return (
                                        <Chip
                                            key={m.id}
                                            label={m.name}
                                            size="small"
                                            sx={{ fontFamily: 'inherit', fontWeight: 600 }}
                                            avatar={<Chip size="small" label={isRetrain ? 'RETRAIN' : 'NEW'} sx={{ height: 16, fontSize: '0.6rem', bgcolor: isRetrain ? '#fde68a' : '#dcfce7', color: isRetrain ? '#b45309' : '#166534' }} />}
                                        />
                                    );
                                })}
                            </Box>
                        </Box>
                    ))}
                </Box>
            </DialogContent>
            <DialogActions sx={{ px: 3, pb: 3 }}>
                <Button onClick={onClose} variant="outlined" sx={{ borderColor: '#cbd5e1', color: '#475569' }}>Cancel</Button>
                <Button onClick={onStart} variant="contained" color="primary" endIcon={<ArrowForwardIcon />}>Start Training</Button>
            </DialogActions>
        </Dialog>
    );
};
