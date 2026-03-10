import { useQuery, useMutation } from '@tanstack/react-query';
import * as api from '../api/upload';
import { useUploadStore } from '../store/uploadStore';

export const useParseFiles = () => {
    return useMutation({
        mutationFn: api.parseFiles
    });
};

export const useTrainMethods = () => {
    const setTrainingActive = useUploadStore(state => state.setTrainingActive);

    return useMutation({
        mutationFn: api.trainMethods,
        onMutate: () => {
            setTrainingActive(true);
        },
        onSuccess: () => {
            // Training request was queued. Don't set false here immediately —
            // actual training is async on the backend. The TopBar status poll
            // (GET /api/training/status every 2s) will set false when done.
        },
        onError: () => {
            setTrainingActive(false);
        }
    });
};

export const useGetFiles = () => {
    return useQuery({
        queryKey: ['files'],
        queryFn: api.getFiles,
        refetchInterval: 15000 // refresh occasionally
    });
};

export const useGetFileMethods = (filename) => {
    return useQuery({
        queryKey: ['fileMethods', filename],
        queryFn: () => api.getFileMethods(filename),
        enabled: !!filename
    });
};

export const useRetrainMethod = () => {
    return useMutation({
        mutationFn: ({ filename, method }) => api.retrainMethod(filename, method)
    });
};
