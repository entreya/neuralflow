import { client } from './client';

export const runQuery = async (filename, query) => {
    const { data } = await client.post('/run', { filename, query });
    return data;
};
