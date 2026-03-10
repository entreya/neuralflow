import { create } from 'zustand';
import axios from 'axios';

const MAX_LOGS = 500;

export const useUploadStore = create((set, get) => ({
    droppedFiles: [],
    fileTree: {}, // [filePath: string]: { filename, fullPath, methods: [] }
    selectedMethods: new Set(), // Set of "filePath::methodName"
    methodStatus: new Map(), // Map<"filePath::methodName", 'not_trained'|'queued'|'processing'|'trained'|'failed'>
    activeFile: null,
    trainingActive: false,

    // ── Log state ──────────────────────────────────────────────
    logs: [],               // array of LogEntry objects
    logConnected: false,    // SSE connection status
    unreadLogs: 0,          // count since user last viewed console
    consoleVisible: false,  // is console tab/panel in view

    // ── Log actions ────────────────────────────────────────────
    addLog: (entry) => set(state => {
        const logs = [...state.logs, entry];
        // prune oldest if over limit
        const trimmed = logs.length > MAX_LOGS ? logs.slice(logs.length - MAX_LOGS) : logs;
        return {
            logs: trimmed,
            // only increment unread if console is not currently visible
            unreadLogs: state.consoleVisible ? 0 : state.unreadLogs + 1,
        };
    }),

    setLogConnected: (v) => set({ logConnected: v }),

    clearLogs: () => set({ logs: [], unreadLogs: 0 }),

    setConsoleVisible: (v) => set(state => ({
        consoleVisible: v,
        unreadLogs: v ? 0 : state.unreadLogs,
    })),

    // ── Training state ─────────────────────────────────────────
    setTrainingActive: (v) => set({ trainingActive: v }),
    setDroppedFiles: (files) => set({ droppedFiles: files }),

    setParsedTree: (tree) => {
        // tree is an array of files returned from /api/parse
        const newFileTree = {};
        const newSelected = new Set();
        const newStatus = new Map();

        tree.forEach(f => {
            newFileTree[f.path] = f;
            f.methods.forEach(m => {
                const id = `${f.path}::${m.name}`;
                newStatus.set(id, m.status);
                if (m.status === 'trained') {
                    newSelected.add(id); // pre-select trained methods
                }
            });
        });

        set({ fileTree: newFileTree, selectedMethods: newSelected, methodStatus: newStatus });
    },

    toggleMethod: (filePath, methodName) => set(state => {
        const id = `${filePath}::${methodName}`;
        const newSelected = new Set(state.selectedMethods);
        if (newSelected.has(id)) {
            newSelected.delete(id);
        } else {
            newSelected.add(id);
        }
        return { selectedMethods: newSelected };
    }),

    // Batch setter: replace all selections for a single file atomically
    setFileSelection: (filePath, selectedNames) => set(state => {
        // Keep all selections from OTHER files, replace this file's selections
        const otherFileSelections = new Set(
            [...state.selectedMethods].filter(id => !id.startsWith(filePath + '::'))
        );
        selectedNames.forEach(name => {
            otherFileSelections.add(`${filePath}::${name}`);
        });
        return { selectedMethods: otherFileSelections };
    }),

    selectAllInFile: (filePath) => set(state => {
        const f = state.fileTree[filePath];
        if (!f) return state;
        const newSelected = new Set(state.selectedMethods);
        f.methods.forEach(m => newSelected.add(`${filePath}::${m.name}`));
        return { selectedMethods: newSelected };
    }),

    deselectAllInFile: (filePath) => set(state => {
        const f = state.fileTree[filePath];
        if (!f) return state;
        const newSelected = new Set(state.selectedMethods);
        f.methods.forEach(m => newSelected.delete(`${filePath}::${m.name}`));
        return { selectedMethods: newSelected };
    }),

    selectAll: () => set(state => {
        const newSelected = new Set();
        Object.values(state.fileTree).forEach(f => {
            f.methods.forEach(m => newSelected.add(`${f.path}::${m.name}`));
        });
        return { selectedMethods: newSelected };
    }),

    setMethodStatus: (filePath, methodName, status) => set(state => {
        const id = `${filePath}::${methodName}`;
        const newMap = new Map(state.methodStatus);
        newMap.set(id, status);
        return { methodStatus: newMap };
    }),

    setActiveFile: (name) => set({ activeFile: name }),

    updateFromSSE: (event) => set(state => {
        const newMap = new Map(state.methodStatus);
        const { type, message, meta } = event;

        // Helper: find which file path contains a given method name
        const findPath = (fnName) => {
            for (const f of Object.values(state.fileTree)) {
                if (f.methods && f.methods.some(m => m.name === fnName)) {
                    return f.path;
                }
            }
            return null;
        };

        // Helper: extract method name from common backend message patterns
        const extractFnName = (msg) => {
            // "Verbalizing feeCalculation()..."
            let m = msg.match(/Verbalizing\s+(\w+)\(\)/);
            if (m) return m[1];
            // "N QA pairs saved for feeCalculation()"
            m = msg.match(/QA pairs saved for\s+(\w+)\(\)/);
            if (m) return m[1];
            // "Verbalized feeCalculation()"
            m = msg.match(/^Verbalized\s+(\w+)\(\)$/);
            if (m) return m[1];
            // "Retrain complete for feeCalculation()"
            m = msg.match(/Retrain complete for\s+(\w+)\(\)/);
            if (m) return m[1];
            // "Generating QA for feeCalculation()..."
            m = msg.match(/Generating QA for\s+(\w+)\(\)/);
            if (m) return m[1];
            // "feeCalculation()" — progress message IS the function name
            m = msg.match(/^(\w+)\(\)$/);
            if (m) return m[1];
            // "Processing queued method: feeCalculation"
            m = msg.match(/Processing queued method:\s+(\w+)/);
            if (m) return m[1];
            return null;
        };

        // Get fnName from meta (queued events) or parse from message
        const fnName = (meta && meta.fnName)
            ? meta.fnName
            : extractFnName(message || '');

        if (!fnName) return state;

        const targetPath = findPath(fnName);
        if (!targetPath) return state;

        const id = `${targetPath}::${fnName}`;

        if (type === 'progress') {
            newMap.set(id, 'processing');
        } else if (type === 'info' && message.startsWith('Verbalizing')) {
            newMap.set(id, 'processing');
        } else if (type === 'ok' && message.includes('QA pairs saved')) {
            newMap.set(id, 'trained');
        } else if (type === 'ok' && message.includes('Retrain complete')) {
            newMap.set(id, 'trained');
        } else if (type === 'error') {
            newMap.set(id, 'failed');
        } else if (type === 'info' && meta && meta.queued) {
            newMap.set(id, 'queued');
        }

        return { methodStatus: newMap };
    }),

    stopTraining: async () => {
        try {
            await axios.post('/api/training/stop');
        } catch (e) {
            console.error('Stop request failed', e);
        }
        set(state => {
            const updated = new Map(state.methodStatus);
            for (const [key, status] of updated) {
                if (status === 'queued' || status === 'processing') {
                    updated.set(key, 'not_trained');
                }
            }
            return { methodStatus: updated, trainingActive: false };
        });
    },

    clearAll: () => set({
        droppedFiles: [],
        fileTree: {},
        selectedMethods: new Set(),
        methodStatus: new Map(),
        activeFile: null,
        trainingActive: false,
        logs: [],
        logConnected: false,
        unreadLogs: 0,
        consoleVisible: false,
    })
}));
