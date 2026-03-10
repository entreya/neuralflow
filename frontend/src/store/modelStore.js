import { create } from 'zustand';

// Predefined presets
export const PRESETS = [
    { id: 'fast', label: '⚡ Fast', model: 'qwen3:4b', thinking: false, temp: 0.0 },
    { id: 'balanced', label: '⚖ Bal', model: 'qwen3:8b', thinking: false, temp: 0.2 },
    { id: 'quality', label: '◈ Q', model: 'llama3', thinking: true, temp: 0.3 }
];

export const useModelStore = create((set) => ({
    activePreset: 'fast', // matches PRESETS[0]
    chatModel: 'qwen3:4b',
    embedModel: 'nomic-embed-text', // Usually static
    thinking: false,
    temperature: 0.0,

    setActivePreset: (presetId) => {
        const p = PRESETS.find(x => x.id === presetId);
        if (p) {
            set({
                activePreset: presetId,
                chatModel: p.model,
                thinking: p.thinking,
                temperature: p.temp
            });
        }
    },

    setAdvancedConfig: (config) => set((state) => ({ ...state, ...config, activePreset: null }))
}));
