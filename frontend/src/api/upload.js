import { client } from './client';

export const parseFiles = async (formData) => {
    const { data } = await client.post('/parse', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
    });
    return data; // { files: [{path, filename, methods: [PHPMethod]}] }
};

export const trainMethods = async (payload) => {
    // body: { files: [{filename, path, methods: ['fn1','fn2']}] }
    const { data } = await client.post('/train', payload);
    return data; // { queued: N }
};

export const getFiles = async () => {
    const { data } = await client.get('/files');
    return data;
};

export const getFileMethods = async (filename) => {
    const { data } = await client.get(`/files/${encodeURIComponent(filename)}/methods`);
    return data;
};

export const retrainMethod = async (filename, method) => {
    const { data } = await client.post('/retrain', { filename, method });
    return data;
};
