import { useState, useCallback, useRef } from 'react';
import { Box, Card, Typography, Button, IconButton, Divider } from '@mui/material';
import { useUploadStore } from '../../store/uploadStore';
import { useParseFiles } from '../../hooks/useUpload';
import { useDropzone } from 'react-dropzone';
import UploadFileIcon from '@mui/icons-material/UploadFile';
import FolderIcon from '@mui/icons-material/Folder';
import InsertDriveFileIcon from '@mui/icons-material/InsertDriveFile';
import CloseIcon from '@mui/icons-material/Close';

export const UploadPanel = () => {
    const { droppedFiles, setDroppedFiles, setParsedTree } = useUploadStore();
    const parseFilesMutation = useParseFiles();
    const [isParsing, setIsParsing] = useState(false);
    const [isFolderDragOver, setIsFolderDragOver] = useState(false);
    const folderInputRef = useRef(null);

    // Shared add logic deduplicating by webkitRelativePath or name
    const addFiles = useCallback((newFiles) => {
        const existing = new Set(droppedFiles.map(f => f.customDisplayPath || f.webkitRelativePath || f.name));
        const fresh = newFiles.filter(f => {
            const path = f.customDisplayPath || f.webkitRelativePath || f.name;
            if (!existing.has(path)) {
                existing.add(path);
                return true;
            }
            return false;
        });
        setDroppedFiles([...droppedFiles, ...fresh]);
    }, [droppedFiles, setDroppedFiles]);

    // LEFT ZONE: FILES
    const onFileDrop = useCallback((acceptedFiles) => {
        addFiles(acceptedFiles);
    }, [addFiles]);

    const { getRootProps: getFileProps, getInputProps: getFileInputProps, isDragActive: isFileDragActive, open: openFileDialog } = useDropzone({
        onDrop: onFileDrop,
        accept: {
            'text/plain': ['.php', '.json', '.txt', '.csv']
        },
        multiple: true,
        useFsAccessApi: false,
        noDirAccess: true,
        noClick: true
    });

    const fileProps = getFileProps({ onClick: openFileDialog });

    // RIGHT ZONE: FOLDER
    const handleFolderClick = () => {
        if (folderInputRef.current) {
            folderInputRef.current.click();
        }
    };

    const handleFolderInput = (e) => {
        const files = [...e.target.files].filter(f => /\.(php|json|txt|csv)$/.test(f.name));
        addFiles(files);
        e.target.value = '';
    };

    async function readEntry(entry, path = '') {
        if (entry.isFile) {
            return new Promise(resolve => {
                entry.file(file => {
                    file.customDisplayPath = path + file.name;
                    resolve([file]);
                });
            });
        }
        if (entry.isDirectory) {
            const reader = entry.createReader();
            const newPath = path + entry.name + '/';
            return new Promise(resolve => {
                reader.readEntries(async entries => {
                    const files = await Promise.all(entries.map(e => readEntry(e, newPath)));
                    resolve(files.flat());
                });
            });
        }
        return [];
    }

    const handleFolderDrop = async (e) => {
        e.preventDefault();
        setIsFolderDragOver(false);
        const items = [...e.dataTransfer.items];
        const entries = items.filter(i => i.kind === 'file').map(i => i.webkitGetAsEntry()).filter(Boolean);
        const files = (await Promise.all(entries.map(e => readEntry(e, '')))).flat();

        const accepted = files.filter(f => /\.(php|json|txt|csv)$/.test(f.name));
        addFiles(accepted);
    };

    const removeFile = (displayPathToRemove) => {
        const remaining = droppedFiles.filter(f => {
            const path = f.customDisplayPath || f.webkitRelativePath || f.name;
            return path !== displayPathToRemove;
        });
        setDroppedFiles(remaining);
    };

    const clearAllFiles = () => {
        setDroppedFiles([]);
    };

    const handleParse = async () => {
        if (droppedFiles.length === 0) return;
        setIsParsing(true);
        try {
            const formData = new FormData();
            droppedFiles.forEach(f => {
                const p = f.customDisplayPath || f.webkitRelativePath || f.name;
                formData.append('files', f, p);
            });

            const res = await parseFilesMutation.mutateAsync(formData);
            if (res && res.files) {
                setParsedTree(res.files);
            }
        } catch (err) {
            console.error("Parse failed", err);
        } finally {
            setIsParsing(false);
        }
    };

    const renderTree = () => {
        if (droppedFiles.length === 0) return null;

        const dirs = {};
        const roots = [];
        droppedFiles.forEach(f => {
            const p = f.customDisplayPath || f.webkitRelativePath || f.name;
            if (p.includes('/')) {
                const parts = p.split('/');
                const dir = parts.slice(0, -1).join('/');
                const name = parts[parts.length - 1];
                if (!dirs[dir]) dirs[dir] = [];
                dirs[dir].push({ name, displayPath: p });
            } else {
                roots.push({ name: p, displayPath: p });
            }
        });

        return (
            <Box sx={{ mt: 3, textAlign: 'left', width: '100%', bgcolor: '#f8fafc', p: 2, borderRadius: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 1 }}>
                    <Typography
                        variant="caption"
                        sx={{ color: '#ef4444', cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}
                        onClick={clearAllFiles}
                    >
                        Clear All
                    </Typography>
                </Box>
                {Object.entries(dirs).map(([dir, files]) => (
                    <Box key={dir} sx={{ mb: 1.5 }}>
                        <Typography variant="body2" sx={{ fontFamily: 'inherit', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 1 }}>
                            <FolderIcon sx={{ fontSize: 16, color: '#64748b' }} /> {dir}/ <span style={{ color: '#94a3b8', fontSize: '0.7rem' }}>({files.length} files)</span>
                        </Typography>
                        {files.map(f => (
                            <Box key={f.displayPath} sx={{ display: 'flex', alignItems: 'center', ml: 3, mb: 0.5 }}>
                                <Typography variant="caption" sx={{ color: '#475569', fontFamily: 'inherit', flex: 1, textOverflow: 'ellipsis', overflow: 'hidden', whiteSpace: 'nowrap' }}>
                                    └── {f.name}
                                </Typography>
                                <IconButton size="small" onClick={(e) => { e.stopPropagation(); removeFile(f.displayPath); }} sx={{ p: 0.25, ml: 1 }}>
                                    <CloseIcon sx={{ fontSize: 14, color: '#94a3b8' }} />
                                </IconButton>
                            </Box>
                        ))}
                    </Box>
                ))}
                {roots.map(f => (
                    <Box key={f.displayPath} sx={{ display: 'flex', alignItems: 'center', mb: 0.5 }}>
                        <Typography variant="body2" sx={{ fontFamily: 'inherit', display: 'flex', alignItems: 'center', gap: 1, flex: 1, textOverflow: 'ellipsis', overflow: 'hidden', whiteSpace: 'nowrap' }}>
                            <InsertDriveFileIcon sx={{ fontSize: 16, color: '#64748b' }} /> {f.name}
                        </Typography>
                        <IconButton size="small" onClick={(e) => { e.stopPropagation(); removeFile(f.displayPath); }} sx={{ p: 0.25, ml: 1 }}>
                            <CloseIcon sx={{ fontSize: 14, color: '#94a3b8' }} />
                        </IconButton>
                    </Box>
                ))}
            </Box>
        );
    };

    const zoneStyle = {
        border: '2px dashed #cbd5e1',
        borderRadius: 2,
        padding: '24px 16px',
        textAlign: 'center',
        cursor: 'pointer',
        transition: 'all 0.15s',
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: '#fff',
        '&:hover': { borderColor: '#1e293b', bgcolor: '#f1f5f9' }
    };

    const dragStyle = {
        borderColor: '#1e293b',
        bgcolor: '#f1f5f9'
    };

    return (
        <Box sx={{ p: 3 }}>
            {/* Always-present hidden input for openFileDialog() to work across both views */}
            <input {...getFileInputProps()} style={{ display: 'none' }} />

            {droppedFiles.length === 0 ? (
                <Box sx={{ display: 'flex', gap: 2, alignItems: 'stretch' }}>
                    {/* LEFT ZONE: FILES */}
                    <Box
                        {...fileProps}
                        sx={[zoneStyle, isFileDragActive && dragStyle]}
                    >
                        <UploadFileIcon sx={{ fontSize: 40, color: '#94a3b8', mb: 1.5 }} />
                        <Typography variant="body2" sx={{ color: '#334155', fontWeight: 600, mb: 0.5 }}>
                            Drop files here
                        </Typography>
                        <Typography variant="caption" sx={{ color: '#64748b', mb: 2 }}>
                            or click to browse
                        </Typography>
                        <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', justifyContent: 'center' }}>
                            {['.php', '.json', '.txt', '.csv'].map(ext => (
                                <Box key={ext} sx={{ bgcolor: '#f8fafc', border: '1px solid #e2e8f0', px: 1, py: 0.25, borderRadius: 1, fontSize: '0.65rem', fontFamily: 'inherit', color: '#475569' }}>
                                    {ext}
                                </Box>
                            ))}
                        </Box>
                    </Box>

                    {/* DIVIDER */}
                    <Box sx={{ display: 'flex', alignItems: 'center', position: 'relative' }}>
                        <Divider orientation="vertical" />
                        <Typography variant="caption" sx={{ position: 'absolute', left: '50%', bgcolor: '#f8fafc', px: 0.5, color: '#94a3b8', transform: 'translateX(-50%)' }}>
                            or
                        </Typography>
                    </Box>

                    {/* RIGHT ZONE: FOLDER */}
                    <Box
                        sx={[zoneStyle, isFolderDragOver && dragStyle]}
                        onClick={handleFolderClick}
                        onDragOver={(e) => { e.preventDefault(); setIsFolderDragOver(true); }}
                        onDragLeave={() => setIsFolderDragOver(false)}
                        onDrop={handleFolderDrop}
                    >
                        <input
                            type="file"
                            webkitdirectory=""
                            mozdirectory=""
                            directory=""
                            multiple
                            style={{ display: 'none' }}
                            ref={folderInputRef}
                            onChange={handleFolderInput}
                        />
                        <FolderIcon sx={{ fontSize: 40, color: '#94a3b8', mb: 1.5 }} />
                        <Typography variant="body2" sx={{ color: '#334155', fontWeight: 600, mb: 0.5 }}>
                            Drop folder here
                        </Typography>
                        <Typography variant="caption" sx={{ color: '#64748b', mb: 2 }}>
                            or click to browse
                        </Typography>
                        <Box sx={{ bgcolor: '#f8fafc', border: '1px solid #e2e8f0', px: 1.5, py: 0.25, borderRadius: 1, fontSize: '0.65rem', fontFamily: 'inherit', color: '#475569' }}>
                            Any directory
                        </Box>
                    </Box>
                </Box>
            ) : (
                <Card sx={{ p: 4, borderRadius: 2, border: '2px solid #16a34a', bgcolor: '#f0fdf4' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', mb: 1, justifyContent: 'center', gap: 1 }}>
                        <UploadFileIcon sx={{ fontSize: 24, color: '#10b981' }} />
                        <Typography variant="body2" sx={{ color: '#0f172a', fontWeight: 600 }}>
                            Ready to parse ({droppedFiles.length} files)
                        </Typography>
                    </Box>

                    {renderTree()}

                    <Box sx={{ display: 'flex', gap: 2, mt: 3, justifyContent: 'center' }}>
                        <Button
                            variant="outlined"
                            size="small"
                            onClick={openFileDialog}
                            sx={{ borderColor: '#cbd5e1', color: '#475569', '&:hover': { borderColor: '#94a3b8', bgcolor: '#fff' }, bgcolor: '#fff' }}
                        >
                            + Add more files
                        </Button>
                        <Button
                            variant="outlined"
                            size="small"
                            onClick={handleFolderClick}
                            sx={{ borderColor: '#cbd5e1', color: '#475569', '&:hover': { borderColor: '#94a3b8', bgcolor: '#fff' }, bgcolor: '#fff' }}
                        >
                            + Add folder
                        </Button>
                    </Box>
                </Card>
            )}

            {/* FOLDER INPUT MUST ALWAYS EXIST IN DOM when active so "Add folder" works */}
            {droppedFiles.length > 0 && (
                <input
                    type="file"
                    webkitdirectory=""
                    mozdirectory=""
                    directory=""
                    multiple
                    style={{ display: 'none' }}
                    ref={folderInputRef}
                    onChange={handleFolderInput}
                />
            )}

            {droppedFiles.length > 0 && (
                <Button
                    variant="contained"
                    fullWidth
                    size="large"
                    onClick={(e) => { e.stopPropagation(); handleParse(); }}
                    disabled={isParsing}
                    sx={{
                        mt: 3, bgcolor: '#0f172a', color: '#fff', '&:hover': { bgcolor: '#1e293b' },
                        '&.Mui-disabled': { bgcolor: '#cbd5e1', color: '#fff' }
                    }}
                >
                    {isParsing ? 'Parsing...' : 'Parse Methods →'}
                </Button>
            )}
        </Box>
    );
};
