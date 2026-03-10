import React, { useEffect, useRef, useState, useCallback } from 'react';
import {
    Box, Stack, Typography, IconButton, Tooltip,
    LinearProgress, Badge, Chip, ToggleButton
} from '@mui/material';
import ArrowRightIcon from '@mui/icons-material/ArrowRight';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import AutorenewIcon from '@mui/icons-material/Autorenew';
import ClearAllIcon from '@mui/icons-material/ClearAll';
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward';
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord';
import { useUploadStore } from '../../store/uploadStore';

// ── Icon + color map per log type ──────────────────────────────────
const LOG_META = {
    info: { Icon: ArrowRightIcon, color: '#64748b' },
    ok: { Icon: CheckCircleOutlineIcon, color: '#16a34a' },
    warn: { Icon: WarningAmberIcon, color: '#d97706' },
    error: { Icon: ErrorOutlineIcon, color: '#dc2626' },
    data: { Icon: InfoOutlinedIcon, color: '#2563eb' },
    progress: { Icon: AutorenewIcon, color: '#0f172a', spin: true },
};

// ── Single log line — memoized to avoid full-list re-renders ──────
const LogLine = React.memo(({ entry }) => {
    const meta = LOG_META[entry.type] || LOG_META.info;
    const { Icon, color, spin } = meta;

    return (
        <Box sx={{ mb: entry.type === 'progress' ? 1.5 : 0.5 }}>
            <Box sx={{ display: 'flex', alignItems: 'flex-start' }}>
                {/* timestamp */}
                <Typography
                    component="span"
                    sx={{
                        fontFamily: "'IBM Plex Mono', monospace",
                        fontSize: '0.68rem',
                        color: '#94a3b8',
                        flexShrink: 0,
                        lineHeight: 1.6,
                        minWidth: 62,
                        mr: 1,
                    }}
                >
                    {entry.time}
                </Typography>

                {/* icon */}
                <Icon
                    sx={{
                        fontSize: 14,
                        color,
                        flexShrink: 0,
                        mt: '2px',
                        mr: 1,
                        animation: spin ? 'spin 2s linear infinite' : 'none',
                        '@keyframes spin': { '100%': { transform: 'rotate(360deg)' } },
                    }}
                />

                {/* message */}
                <Typography
                    component="span"
                    sx={{
                        fontFamily: "'IBM Plex Mono', monospace",
                        fontSize: '0.72rem',
                        color: entry.type === 'error' ? '#dc2626'
                            : entry.type === 'warn' ? '#92400e'
                                : entry.type === 'ok' ? '#15803d'
                                    : '#334155',
                        lineHeight: 1.6,
                        wordBreak: 'break-word',
                    }}
                >
                    {entry.message}
                </Typography>
            </Box>

            {/* progress bar — only for 'progress' type with meta */}
            {entry.type === 'progress' && entry.meta?.total > 0 && (
                <Box sx={{ pl: '74px', pr: 2, pt: 0.5 }}>
                    <LinearProgress
                        variant="determinate"
                        value={(entry.meta.current / entry.meta.total) * 100}
                        sx={{
                            height: 4,
                            borderRadius: 2,
                            bgcolor: '#e2e8f0',
                            '& .MuiLinearProgress-bar': { bgcolor: '#0f172a', borderRadius: 2 },
                        }}
                    />
                </Box>
            )}
        </Box>
    );
});

// ── ConsolePanel reads directly from Zustand store ────────────────
// useLogs() is called from App.jsx (once globally), NOT here.
export const ConsolePanel = () => {
    const logs = useUploadStore(s => s.logs);
    const logConnected = useUploadStore(s => s.logConnected);
    const unreadLogs = useUploadStore(s => s.unreadLogs);
    const clearLogs = useUploadStore(s => s.clearLogs);
    const setVisible = useUploadStore(s => s.setConsoleVisible);

    const boxRef = useRef(null);
    const [autoScroll, setAutoScroll] = useState(true);
    const prevLogsLength = useRef(logs.length);

    // Tell store this panel is visible (clears unread count)
    useEffect(() => {
        setVisible(true);
        return () => setVisible(false);
    }, []);

    // Auto-scroll when new logs arrive
    useEffect(() => {
        if (logs.length > prevLogsLength.current && autoScroll && boxRef.current) {
            boxRef.current.scrollTop = boxRef.current.scrollHeight;
        }
        prevLogsLength.current = logs.length;
    }, [logs, autoScroll]);

    // Detect manual scroll up → disable autoscroll
    const handleScroll = useCallback(() => {
        if (!boxRef.current) return;
        const { scrollTop, scrollHeight, clientHeight } = boxRef.current;
        const atBottom = scrollHeight - scrollTop - clientHeight < 40;
        if (atBottom && !autoScroll) {
            setAutoScroll(true);
        } else if (!atBottom && autoScroll) {
            setAutoScroll(false);
        }
    }, [autoScroll]);

    const jumpToBottom = () => {
        if (boxRef.current) {
            boxRef.current.scrollTop = boxRef.current.scrollHeight;
        }
        setAutoScroll(true);
    };

    return (
        <Box
            sx={{
                border: '1px solid #e2e8f0',
                borderRadius: 2,
                mx: 3,
                mb: 3,
                overflow: 'hidden',
                background: '#ffffff',
                display: 'flex',
                flexDirection: 'column',
                height: 240,
                flexShrink: 0,
            }}
        >
            {/* ── Header ── */}
            <Box
                sx={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    p: 1,
                    px: 2,
                    borderBottom: '1px solid #e2e8f0',
                    bgcolor: '#f8fafc',
                    flexShrink: 0,
                }}
            >
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    {/* Connection dot */}
                    <FiberManualRecordIcon
                        sx={{
                            fontSize: 10,
                            color: logConnected ? '#16a34a' : '#94a3b8',
                            mr: 0.75,
                            animation: logConnected ? 'pulse 2s infinite' : 'none',
                            '@keyframes pulse': {
                                '0%,100%': { opacity: 1 },
                                '50%': { opacity: 0.4 },
                            },
                        }}
                    />
                    <Badge
                        badgeContent={unreadLogs > 0 ? unreadLogs : null}
                        color="error"
                        sx={{ '& .MuiBadge-badge': { fontSize: '0.6rem', height: 14, minWidth: 14 } }}
                    >
                        <Typography
                            variant="body2"
                            sx={{
                                fontFamily: "'IBM Plex Mono', monospace",
                                fontWeight: 700,
                                fontSize: '0.75rem',
                            }}
                        >
                            Console
                        </Typography>
                    </Badge>
                </Box>

                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                    <ToggleButton
                        value="check"
                        selected={autoScroll}
                        onChange={() => setAutoScroll(v => !v)}
                        size="small"
                        sx={{ px: 1, py: 0.5, border: 'none', height: 28 }}
                    >
                        <ArrowDownwardIcon sx={{ fontSize: 16, mr: 0.5 }} />
                        <span style={{ fontSize: '0.7rem', fontWeight: 600 }}>Autoscroll</span>
                    </ToggleButton>
                    <IconButton size="small" onClick={clearLogs} title="Clear">
                        <ClearAllIcon sx={{ fontSize: 18 }} />
                    </IconButton>
                </Box>
            </Box>

            {/* ── Log body ── */}
            <Box
                ref={boxRef}
                onScroll={handleScroll}
                sx={{
                    flex: 1,
                    overflowY: 'auto',
                    px: 1.5,
                    py: 1,
                    bgcolor: '#f8fafc',
                    fontFamily: "'IBM Plex Mono', monospace",
                    fontSize: '0.72rem',
                    position: 'relative',
                    '&::-webkit-scrollbar': { width: 4 },
                    '&::-webkit-scrollbar-track': { background: 'transparent' },
                    '&::-webkit-scrollbar-thumb': { background: '#e2e8f0', borderRadius: 2 },
                }}
            >
                {logs.length === 0 ? (
                    <Typography
                        variant="body2"
                        sx={{
                            color: '#94a3b8',
                            textAlign: 'center',
                            mt: 8,
                            fontFamily: "'IBM Plex Mono', monospace",
                        }}
                    >
                        Console output will appear here during training and queries
                    </Typography>
                ) : (
                    logs.map((entry, i) => <LogLine key={i} entry={entry} />)
                )}

                {/* Floating jump-to-bottom chip */}
                {!autoScroll && unreadLogs > 0 && (
                    <Box
                        sx={{
                            position: 'sticky',
                            bottom: 8,
                            display: 'flex',
                            justifyContent: 'center',
                        }}
                    >
                        <Chip
                            label={`↓ ${unreadLogs} new`}
                            size="small"
                            onClick={jumpToBottom}
                            sx={{
                                fontFamily: "'IBM Plex Mono', monospace",
                                fontSize: '0.68rem',
                                fontWeight: 700,
                                background: '#0f172a',
                                color: '#fff',
                                cursor: 'pointer',
                                '&:hover': { background: '#1e293b' },
                            }}
                        />
                    </Box>
                )}
            </Box>
        </Box>
    );
};
