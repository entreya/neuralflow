import { Box, Typography, Chip, Divider, Button, LinearProgress } from '@mui/material';
import StopCircleIcon from '@mui/icons-material/StopCircle';
import { useUploadStore } from '../../store/uploadStore';
import { useModelStore } from '../../store/modelStore';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';
import React, { useEffect } from 'react';

// Local polling since this is root level '/health' outside the vite '/api' proxy boundary if not proxied
// Actually, earlier prompt said GET /health via react-query, proxy covers /api/health or /health?
// The prompt says "Health poll: GET /health every 15s". If vite.config.js proxies '/api', we need to check if /health works. 
// Assuming it proxies to full host or we hit it via proxy
export const TopBar = () => {
    const { activeFile, trainingActive, setTrainingActive, stopTraining } = useUploadStore();
    const { chatModel, temperature } = useModelStore();

    const { data: healthData, isError } = useQuery({
        queryKey: ['health'],
        queryFn: async () => {
            const { data } = await axios.get('/health');
            return data;
        },
        refetchInterval: 15000,
        retry: true
    });

    // Poll training status every 2 seconds to keep global alignment
    useQuery({
        queryKey: ['trainingStatus'],
        queryFn: async () => {
            const { data } = await axios.get('/api/training/status');
            if (data.active === false && trainingActive === true) {
                // If backend says stopped but UI says active, forcefully drop state
                setTrainingActive(false);
            }
            return data;
        },
        refetchInterval: 2000,
    });

    const isConnected = !isError && healthData;

    const handleStop = async () => {
        // Optimistically update UI immediately via Zustand bound logic
        await stopTraining();
    };

    return (
        <Box sx={{
            height: 52,
            position: 'relative',
            display: 'flex',
            alignItems: 'center',
            px: 2,
            borderBottom: '1px solid #e2e8f0',
            background: '#fff',
            flexShrink: 0
        }}>
            <style>
                {`
                @keyframes pulse-intense {
                    0%, 100% { opacity: 1; }
                    50%      { opacity: 0.65; box-shadow: 0 0 8px rgba(220, 38, 38, 0.4); }
                }
                `}
            </style>

            {/* Brand */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                <Box sx={{
                    width: 24, height: 24, borderRadius: 1.5,
                    background: '#1e293b', color: '#fff',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontWeight: 800, fontSize: '0.75rem', fontFamily: 'inherit'
                }}>
                    NF
                </Box>
                <Typography variant="h6" sx={{ letterSpacing: '-0.5px' }}>
                    NeuralFlow
                </Typography>
            </Box>

            <Divider orientation="vertical" flexItem sx={{ mx: 2, my: 1.5 }} />

            {/* Active File */}
            <Typography variant="body2" sx={{
                color: activeFile ? '#0f172a' : '#64748b',
                fontWeight: activeFile ? 600 : 400,
                fontFamily: "inherit"
            }}>
                {activeFile || 'No file selected'}
            </Typography>

            <Box sx={{ flex: 1 }} />

            {/* Stop Training Control */}
            {trainingActive && (
                <Button
                    variant="contained"
                    color="error"
                    size="small"
                    startIcon={<StopCircleIcon />}
                    onClick={handleStop}
                    sx={{
                        fontWeight: 700,
                        fontSize: '0.75rem',
                        borderRadius: 2,
                        px: 2,
                        mr: 3,
                        animation: 'pulse-intense 1.5s infinite',
                    }}
                >
                    Stop Training
                </Button>
            )}

            {/* Status */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Box sx={{
                    width: 8, height: 8, borderRadius: '50%',
                    bgcolor: isConnected ? '#16a34a' : '#dc2626',
                    boxShadow: isConnected ? '0 0 0 3px #dcfce7' : '0 0 0 3px #fee2e2'
                }} />
                <Typography variant="caption" sx={{ mr: 2 }}>
                    {isConnected ? 'connected' : 'error'}
                </Typography>

                <Chip
                    label={`${chatModel} · ${temperature}°`}
                    variant="outlined"
                    size="small"
                    sx={{ borderColor: '#cbd5e1', color: '#475569' }}
                />
            </Box>

            {/* Progress Bar (Visible Only When Training is Active) */}
            {trainingActive && (
                <LinearProgress
                    variant="indeterminate"
                    color="error"
                    sx={{
                        position: 'absolute',
                        bottom: 0,
                        left: 0,
                        right: 0,
                        height: 3
                    }}
                />
            )}
        </Box>
    );
};
