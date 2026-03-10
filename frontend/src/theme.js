import { createTheme } from '@mui/material/styles';

export const theme = createTheme({
    palette: {
        mode: 'light',
        primary: { main: '#1e293b' },   // slate-900 (main actions)
        secondary: { main: '#475569' },   // slate-600
        success: { main: '#16a34a' },   // green-600
        warning: { main: '#d97706' },   // amber-600
        error: { main: '#dc2626' },   // red-600
        info: { main: '#2563eb' },   // blue-600
        background: {
            default: '#f8fafc',              // slate-50
            paper: '#ffffff'
        },
        text: {
            primary: '#0f172a',            // slate-950
            secondary: '#64748b',            // slate-500
            disabled: '#cbd5e1'             // slate-300
        }
    },
    typography: {
        fontFamily: "'Inter', sans-serif",
        h5: { fontWeight: 700, fontSize: '1.1rem', color: '#0f172a' },
        h6: { fontWeight: 700, fontSize: '0.9rem', color: '#0f172a' },
        body2: { fontSize: '0.8rem' },
        caption: { fontSize: '0.72rem', color: '#64748b' }
    },
    shape: {
        borderRadius: 8
    },
    components: {
        MuiCard: {
            defaultProps: { elevation: 0 },
            styleOverrides: {
                root: { border: '1px solid #e2e8f0', borderRadius: 10 }
            }
        },
        MuiButton: {
            defaultProps: { disableElevation: true },
            styleOverrides: {
                root: { textTransform: 'none', fontWeight: 600, borderRadius: 7 },
                containedPrimary: {
                    background: '#1e293b',
                    color: '#fff',
                    '&:hover': { background: '#0f172a' }
                }
            }
        },
        MuiChip: {
            styleOverrides: {
                root: {
                    borderRadius: 6,
                    fontFamily: "'Inter', sans-serif",
                    fontSize: '0.7rem',
                    fontWeight: 600
                }
            }
        },
        MuiAccordion: {
            defaultProps: { elevation: 0 },
            styleOverrides: {
                root: {
                    border: '1px solid #e2e8f0',
                    borderRadius: '8px !important',
                    marginBottom: 8,
                    '&:before': { display: 'none' },
                    '&.Mui-expanded': { margin: '0 0 8px 0' }
                }
            }
        },
        MuiAccordionSummary: {
            styleOverrides: {
                root: {
                    background: '#f8fafc',
                    borderRadius: 8,
                    minHeight: 48,
                    fontWeight: 600
                }
            }
        },
        MuiDataGrid: {
            styleOverrides: {
                root: {
                    border: 'none',
                    fontFamily: "'Inter', sans-serif",
                    fontSize: '0.78rem'
                },
                columnHeaders: { background: '#f1f5f9', borderRadius: 0 },
                row: { '&.Mui-selected': { background: '#f0fdf4' } }
            }
        },
        MuiLinearProgress: {
            styleOverrides: {
                root: { borderRadius: 4, height: 5 },
                bar: { background: '#1e293b' }
            }
        },
        MuiTab: {
            styleOverrides: {
                root: {
                    textTransform: 'none',
                    fontWeight: 600,
                    fontFamily: "'Inter', sans-serif",
                    minHeight: 44
                }
            }
        },
        MuiDivider: {
            styleOverrides: {
                root: { borderColor: '#e2e8f0' }
            }
        }
    }
});
