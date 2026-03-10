import { Box, Typography, Paper, IconButton, Tooltip, Table, TableBody, TableCell, TableRow, TableHead, TableFooter, Chip } from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vs } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { ScoreBadge } from './ScoreBadge';
import { useModelStore } from '../../store/modelStore';
import { useState } from 'react';

export const OutputPanel = ({ result }) => {
    const { chatModel } = useModelStore();
    const [copied, setCopied] = useState(false);

    if (!result) return null;

    const handleCopy = () => {
        navigator.clipboard.writeText(JSON.stringify(result.json, null, 2));
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    const components = result.json?.components || [];
    const hasComponents = components.length > 0;
    let total = 0;
    if (hasComponents) {
        total = components.reduce((acc, curr) => acc + (typeof curr.amount === 'number' ? curr.amount : 0), 0);
    }

    return (
        <Box sx={{ p: 3, pt: 0, display: 'flex', flexDirection: 'column', gap: 2, flex: 1, overflow: 'hidden' }}>

            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="h6">Output</Typography>
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
                    <Chip label={chatModel} size="small" variant="outlined" sx={{ color: '#64748b' }} />
                    {result.retries > 0 && <Chip label={`${result.retries} retries`} size="small" variant="outlined" color="warning" />}
                    <ScoreBadge score={result.score} />
                </Box>
            </Box>

            {/* JSON Block with copy button */}
            <Paper elevation={0} sx={{
                position: 'relative',
                bgcolor: '#f8fafc',
                border: '1px solid #e2e8f0',
                borderRadius: 2,
                overflow: 'hidden',
                display: 'flex',
                flexDirection: 'column',
                maxHeight: 320
            }}>
                <Tooltip title={copied ? "Copied!" : "Copy JSON"} placement="left">
                    <IconButton
                        size="small"
                        onClick={handleCopy}
                        sx={{ position: 'absolute', top: 8, right: 8, bgcolor: 'rgba(255,255,255,0.8)', '&:hover': { bgcolor: '#fff' } }}
                    >
                        <ContentCopyIcon sx={{ fontSize: 16 }} />
                    </IconButton>
                </Tooltip>

                <Box sx={{ overflowY: 'auto', flex: 1 }}>
                    <SyntaxHighlighter
                        language="json"
                        style={vs}
                        customStyle={{
                            margin: 0,
                            padding: '16px',
                            background: 'transparent',
                            fontSize: '0.85rem',
                            fontFamily: "inherit"
                        }}
                    >
                        {JSON.stringify(result.json, null, 2)}
                    </SyntaxHighlighter>
                </Box>
            </Paper>

            {/* Components Table */}
            {hasComponents && (
                <Paper elevation={0} sx={{ border: '1px solid #e2e8f0', borderRadius: 2, overflow: 'hidden' }}>
                    <Table size="small" sx={{ '& .MuiTableCell-root': { fontSize: '0.85rem', borderColor: '#e2e8f0' } }}>
                        <TableHead sx={{ bgcolor: '#f1f5f9' }}>
                            <TableRow>
                                <TableCell>#</TableCell>
                                <TableCell>Message</TableCell>
                                <TableCell align="right">Amount</TableCell>
                                <TableCell>Currency</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {components.map((c, i) => (
                                <TableRow key={i}>
                                    <TableCell>{i + 1}</TableCell>
                                    <TableCell>{c.message}</TableCell>
                                    <TableCell align="right" sx={{ fontWeight: 600 }}>{c.amount}</TableCell>
                                    <TableCell>{result.json.currency || 'INR'}</TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                        <TableFooter sx={{ bgcolor: '#f8fafc' }}>
                            <TableRow>
                                <TableCell colSpan={2} sx={{ fontWeight: 700 }}>Total</TableCell>
                                <TableCell align="right" sx={{ fontWeight: 700, color: '#0f172a' }}>{total}</TableCell>
                                <TableCell sx={{ fontWeight: 700 }}>{result.json.currency || 'INR'}</TableCell>
                            </TableRow>
                        </TableFooter>
                    </Table>
                </Paper>
            )}

        </Box>
    );
};
