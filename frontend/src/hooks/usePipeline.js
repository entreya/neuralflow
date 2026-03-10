import { useMutation } from '@tanstack/react-query';
import * as api from '../api/pipeline';

export const useRunPipeline = () => {
    return useMutation({
        mutationFn: ({ filename, query }) => api.runQuery(filename, query)
    });
};
