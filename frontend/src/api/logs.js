export const createLogStream = () => {
    // Uses Vite proxy /api
    return new EventSource('/api/logs');
};
