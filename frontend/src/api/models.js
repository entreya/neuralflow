import { client } from './client';

export const getModels = async () => {
    const { data } = await client.get('/models');
    return data;
};

export const getConfig = async () => {
    const { data } = await client.get('/config');
    return data;
};

export const setConfig = async (configData) => {
    const { data } = await client.post('/config', configData);
    return data;
};
