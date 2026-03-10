import { Box } from '@mui/material';
import { TopBar } from './TopBar';

export const AppShell = ({ leftPanel, centerPanel, rightPanel }) => {
    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh', overflow: 'hidden', bgcolor: '#f8fafc' }}>
            <TopBar />

            <Box sx={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
                {/* LEFT */}
                <Box sx={{
                    width: 280,
                    flexShrink: 0,
                    borderRight: '1px solid #e2e8f0',
                    bgcolor: '#fff',
                    overflowY: 'auto',
                    p: 2
                }}>
                    {leftPanel}
                </Box>

                {/* CENTER */}
                <Box sx={{
                    flex: 1,
                    display: 'flex',
                    flexDirection: 'column',
                    overflow: 'hidden',
                    position: 'relative'
                }}>
                    {centerPanel}
                </Box>

                {/* RIGHT */}
                <Box sx={{
                    width: 320,
                    flexShrink: 0,
                    borderLeft: '1px solid #e2e8f0',
                    bgcolor: '#fff',
                    display: 'flex',
                    flexDirection: 'column',
                    overflow: 'hidden'
                }}>
                    {rightPanel}
                </Box>
            </Box>
        </Box>
    );
};
