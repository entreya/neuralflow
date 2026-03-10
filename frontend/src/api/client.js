import axios from 'axios';

export const client = axios.create({
    baseURL: '/api',
    timeout: 300000 // 5 minutes for training operations
});

client.interceptors.response.use(
    (response) => response,
    (error) => {
        console.error('[API Error]', error);
        return Promise.reject(error);
    }
);
