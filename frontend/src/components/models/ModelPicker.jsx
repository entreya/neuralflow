import { Box, Card, Typography, ToggleButtonGroup, ToggleButton, Accordion, AccordionSummary, AccordionDetails, Select, MenuItem, Switch, Slider, Button, Tooltip } from '@mui/material';
import ArrowRightIcon from '@mui/icons-material/ArrowRight';
import { useModelStore, PRESETS } from '../../store/modelStore';
import { useSetConfig } from '../../hooks/useModels';
import { useState } from 'react';

export const ModelPicker = () => {
    const store = useModelStore();
    const setConfig = useSetConfig();
    const [expanded, setExpanded] = useState(false);

    // Local state for advanced config
    const [advModel, setAdvModel] = useState(store.chatModel);
    const [advEmbed, setAdvEmbed] = useState(store.embedModel);
    const [advThink, setAdvThink] = useState(store.thinking);
    const [advTemp, setAdvTemp] = useState(store.temperature);

    const handlePresetSelect = (e, newPresetId) => {
        if (!newPresetId) return;
        store.setActivePreset(newPresetId);

        const p = PRESETS.find(x => x.id === newPresetId);
        if (p) {
            setConfig.mutate({
                model: p.model,
                embedModel: 'nomic-embed-text',
                thinking: p.thinking,
                temperature: p.temp
            });
            setExpanded(false);
        }
    };

    const handleApplyAdvanced = () => {
        store.setAdvancedConfig({
            chatModel: advModel,
            embedModel: advEmbed,
            thinking: advThink,
            temperature: advTemp
        });
        setConfig.mutate({
            model: advModel,
            embedModel: advEmbed,
            thinking: advThink,
            temperature: advTemp
        });
        setExpanded(false);
    };

    return (
        <Box>
            <Typography variant="h6" sx={{ mb: 2 }}>Model Settings</Typography>

            <Typography variant="caption" sx={{ display: 'block', mb: 1, fontWeight: 700 }}>PRESETS</Typography>
            <ToggleButtonGroup
                value={store.activePreset || ''}
                exclusive
                onChange={handlePresetSelect}
                fullWidth
                size="small"
                sx={{ mb: 3 }}
            >
                {PRESETS.map(p => (
                    <Tooltip key={p.id} title={p.model === 'llama3' ? 'Requires llama3' : ''}>
                        <ToggleButton
                            value={p.id}
                            sx={{
                                '&.Mui-selected': { bgcolor: '#1e293b', color: '#fff', '&:hover': { bgcolor: '#0f172a' } },
                                fontFamily: "'IBM Plex Mono', monospace",
                                fontWeight: 600,
                                fontSize: '0.75rem'
                            }}
                        >
                            {p.label}
                        </ToggleButton>
                    </Tooltip>
                ))}
            </ToggleButtonGroup>

            <Accordion expanded={expanded} onChange={(e, isExp) => setExpanded(isExp)}>
                <AccordionSummary expandIcon={<ArrowRightIcon sx={{ transform: expanded ? 'rotate(90deg)' : 'none' }} />}>
                    <Typography variant="body2" sx={{ fontFamily: 'inherit' }}>Advanced Config</Typography>
                </AccordionSummary>
                <AccordionDetails sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>

                    <Box>
                        <Typography variant="caption" sx={{ display: 'block', mb: 0.5 }}>Chat Model</Typography>
                        <Select size="small" fullWidth value={advModel} onChange={e => setAdvModel(e.target.value)} sx={{ fontSize: '0.8rem' }}>
                            <MenuItem value="qwen3:4b">qwen3:4b</MenuItem>
                            <MenuItem value="qwen3:8b">qwen3:8b</MenuItem>
                            <MenuItem value="llama3">llama3</MenuItem>
                        </Select>
                    </Box>

                    <Box>
                        <Typography variant="caption" sx={{ display: 'block', mb: 0.5 }}>Embed Model</Typography>
                        <Select size="small" fullWidth value={advEmbed} onChange={e => setAdvEmbed(e.target.value)} sx={{ fontSize: '0.8rem' }}>
                            <MenuItem value="nomic-embed-text">nomic-embed-text</MenuItem>
                        </Select>
                    </Box>

                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Typography variant="caption">Thinking</Typography>
                        <Switch size="small" checked={advThink} onChange={e => setAdvThink(e.target.checked)} />
                    </Box>

                    <Box>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                            <Typography variant="caption">Temperature</Typography>
                            <Typography variant="caption">{advTemp.toFixed(1)}</Typography>
                        </Box>
                        <Slider
                            value={advTemp}
                            onChange={(e, v) => setAdvTemp(v)}
                            min={0} max={1} step={0.1}
                            size="small"
                            sx={{ color: '#1e293b' }}
                        />
                    </Box>

                    <Button variant="contained" color="primary" onClick={handleApplyAdvanced} sx={{ mt: 1 }}>
                        Apply
                    </Button>
                </AccordionDetails>
            </Accordion>

            <Card sx={{ mt: 3, p: 2, bgcolor: '#f8fafc' }}>
                <Typography variant="caption" sx={{ display: 'block', color: '#94a3b8', mb: 1 }}>Active:</Typography>
                <Typography variant="body2" sx={{ fontFamily: 'inherit', fontWeight: 600 }}>
                    {store.chatModel} · temp {store.temperature.toFixed(1)}
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: 'inherit', color: '#64748b', mt: 0.5 }}>
                    Thinking: {store.thinking ? 'on' : 'off'}
                </Typography>
            </Card>
        </Box>
    );
};
