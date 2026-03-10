import { useQuery, useMutation } from '@tanstack/react-query';
import * as api from '../api/models';

export const useGetModels = () => {
    return useQuery({
        queryKey: ['models'],
        queryFn: api.getModels,
        retry: false
    });
};

export const useGetConfig = () => {
    return useQuery({
        queryKey: ['config'],
        queryFn: api.getConfig,
        retry: false
    });
};

export const useSetConfig = () => {
    return useMutation({
        mutationFn: api.setConfig
    });
};
