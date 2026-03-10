import { useState } from 'react';
import { Box, Tabs, Tab, Typography, Chip, Accordion, AccordionSummary, AccordionDetails } from '@mui/material';
import ArrowRightIcon from '@mui/icons-material/ArrowRight';
import { AppShell } from './components/layout/AppShell';
import { ModelPicker } from './components/models/ModelPicker';
import { UploadPanel } from './components/upload/UploadPanel';
import { MethodAccordion } from './components/upload/MethodAccordion';
import { TrainingProgress } from './components/training/TrainingProgress';
import { TrainingConfirmDialog } from './components/training/TrainingConfirmDialog';
import { QueryPanel } from './components/pipeline/QueryPanel';
import { ConsolePanel } from './components/console/ConsolePanel';
import { OutputPanel } from './components/pipeline/OutputPanel';
import { useUploadStore } from './store/uploadStore';
import { useTrainMethods } from './hooks/useUpload';
import { useRunPipeline } from './hooks/usePipeline';
import { useLogs } from './hooks/useLogs';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';

function App() {
    useLogs(); // Global SSE connection — lives for entire app lifetime
    const [centerTab, setCenterTab] = useState(0); // 0=Upload, 1=Query
    const [rightTab, setRightTab] = useState(0);   // 0=Rules, 1=Corrections
    const [confirmOpen, setConfirmOpen] = useState(false);

    const { selectedMethods, fileTree, setMethodStatus } = useUploadStore();
    const trainMutation = useTrainMethods();
    const runMutation = useRunPipeline();
    const [pipelineResult, setPipelineResult] = useState(null);

    // Derive training payload
    const handleStartTraining = async () => {
        setConfirmOpen(false);
        setCenterTab(1); // Switch to query/console seamlessly

        // Build Payload => files: [{filename, path, methods: []}]
        const filesMap = {};
        selectedMethods.forEach(id => {
            const [path, name] = id.split('::');
            if (!filesMap[path]) {
                const fileObj = fileTree[path];
                filesMap[path] = { filename: fileObj.filename, path, methods: [] };
            }
            filesMap[path].methods.push(name);
            setMethodStatus(path, name, 'queued'); // Optimistically queue
        });

        const payload = { files: Object.values(filesMap) };
        try {
            await trainMutation.mutateAsync(payload);
        } catch (err) {
            console.error("Training failed to submit", err);
        }
    };

    const activeFile = useUploadStore(state => state.activeFile);

    // Right Panel queries
    const { data: rulesData } = useQuery({
        queryKey: ['rules', activeFile],
        queryFn: async () => {
            if (!activeFile) return null;
            const { data } = await axios.get('/api/rules?filename=' + encodeURIComponent(activeFile));
            return data;
        },
        enabled: rightTab === 0 && !!activeFile
    });

    const { data: corrData } = useQuery({
        queryKey: ['corrections', activeFile],
        queryFn: async () => {
            if (!activeFile) return null;
            const { data } = await axios.get('/api/corrections?filename=' + encodeURIComponent(activeFile));
            return data;
        },
        enabled: rightTab === 1 && !!activeFile
    });

    return (
        <AppShell
            leftPanel={<ModelPicker />}

            centerPanel={
                <>
                    <Box sx={{ borderBottom: 1, borderColor: 'divider', bgcolor: '#fff' }}>
                        <Tabs value={centerTab} onChange={(e, v) => setCenterTab(v)} textColor="primary" indicatorColor="primary">
                            <Tab label="Upload & Train" />
                            <Tab label="Query Pipeline" />
                        </Tabs>
                    </Box>

                    {centerTab === 0 && (
                        <Box sx={{ flex: 1, overflowY: 'auto', p: 0, position: 'relative' }}>
                            <UploadPanel />
                            <Box sx={{ px: 3, pb: 10 }}>
                                <MethodAccordion />
                            </Box>
                            <TrainingProgress onBegin={() => setConfirmOpen(true)} />
                        </Box>
                    )}

                    {centerTab === 1 && (
                        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
                            <QueryPanel
                                isRunning={runMutation.isPending}
                                onRun={async (q) => {
                                    setPipelineResult(null);
                                    try {
                                        const res = await runMutation.mutateAsync({ filename: activeFile, query: q });
                                        setPipelineResult(res);
                                    } catch (e) {
                                        console.error("Pipeline run error", e);
                                    }
                                }}
                            />
                            <ConsolePanel />
                            <OutputPanel result={pipelineResult} />
                        </Box>
                    )}

                    <TrainingConfirmDialog
                        open={confirmOpen}
                        onClose={() => setConfirmOpen(false)}
                        onStart={handleStartTraining}
                    />
                </>
            }

            rightPanel={
                <>
                    <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                        <Tabs value={rightTab} onChange={(e, v) => setRightTab(v)} textColor="primary" indicatorColor="primary">
                            <Tab label="Rules" />
                            <Tab label="Corrections" />
                        </Tabs>
                    </Box>
                    <Box sx={{ flex: 1, overflowY: 'auto', p: 2 }}>
                        {!activeFile ? (
                            <Typography variant="body2" sx={{ color: '#64748b', mt: 4, textAlign: 'center' }}>
                                Upload and train a file to see extracted rules
                            </Typography>
                        ) : (
                            // RULE LIST RENDERING
                            rightTab === 0 ? (
                                rulesData?.data ? (
                                    rulesData.data.map((r, i) => (
                                        <Box key={i} sx={{ mb: 2, p: 1.5, border: '1px solid #e2e8f0', borderRadius: 2, bgcolor: '#f8fafc' }}>
                                            <Typography variant="caption" sx={{ fontFamily: 'inherit', display: 'block', mb: 0.5, color: '#64748b' }}>{r.rule_id}</Typography>
                                            <Typography variant="body2" sx={{ fontWeight: 600, mb: 1 }}>{r.description}</Typography>
                                            <Box sx={{ display: 'flex', gap: 1 }}>
                                                <Chip label={r.type} size="small" sx={{ height: 18, fontSize: '0.65rem' }} />
                                                <Chip label={r.severity} size="small" color={r.severity === 'error' ? 'error' : 'warning'} sx={{ height: 18, fontSize: '0.65rem' }} />
                                            </Box>
                                        </Box>
                                    ))
                                ) : <Typography variant="caption" sx={{ color: '#94a3b8' }}>Loading rules...</Typography>
                            ) : (
                                // CORRECTIONS LIST RENDERING
                                corrData?.data ? (
                                    corrData.data.map((c, i) => (
                                        <Box key={i} sx={{ mb: 2, p: 1.5, border: '1px solid #fee2e2', borderRadius: 2, bgcolor: '#fef2f2' }}>
                                            <Typography variant="caption" sx={{ color: '#64748b', display: 'block', mb: 0.5 }}>{new Date(c.created_at).toLocaleString()}</Typography>
                                            <Typography variant="body2" sx={{ fontStyle: 'italic', mb: 1, color: '#475569' }}>"{c.query_excerpt}"</Typography>
                                            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mb: 1 }}>
                                                <Chip label={c.error_type} color="error" variant="outlined" size="small" sx={{ height: 20, fontSize: '0.65rem' }} />
                                            </Box>
                                            <Typography variant="body2" sx={{ color: '#1e293b', fontSize: '0.75rem' }}>{c.correction_text}</Typography>
                                        </Box>
                                    ))
                                ) : <Typography variant="caption" sx={{ color: '#94a3b8' }}>Loading corrections...</Typography>
                            )
                        )}
                    </Box>
                </>
            }
        />
    );
}

export default App;
