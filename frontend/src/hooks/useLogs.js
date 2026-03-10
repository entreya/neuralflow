import { useEffect, useRef } from 'react';
import { useUploadStore } from '../store/uploadStore';

const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 15000]; // ms

/**
 * useLogs — Establishes a global SSE connection to GET /api/logs.
 *
 * MUST be called once in App.jsx (top-level) so the EventSource
 * persists across tab switches. Log entries are pushed into
 * Zustand's uploadStore so any component can read them.
 */
export function useLogs() {
    const addLog = useUploadStore(s => s.addLog);
    const setConnected = useUploadStore(s => s.setLogConnected);
    const updateFromSSE = useUploadStore(s => s.updateFromSSE);
    const retryCount = useRef(0);
    const esRef = useRef(null);
    const unmounted = useRef(false);

    useEffect(() => {
        unmounted.current = false;

        function connect() {
            if (unmounted.current) return;

            // Close any existing connection first
            if (esRef.current) {
                esRef.current.close();
                esRef.current = null;
            }

            const es = new EventSource('/api/logs');
            esRef.current = es;

            es.onopen = () => {
                retryCount.current = 0;
                setConnected(true);
                addLog({
                    time: new Date().toLocaleTimeString('en', { hour12: false }),
                    type: 'info',
                    message: 'Console connected to server log stream',
                });
            };

            // CRITICAL: listen for named 'log' event, not onmessage
            es.addEventListener('log', (e) => {
                try {
                    const entry = JSON.parse(e.data);
                    addLog(entry);
                    updateFromSSE(entry); // Tie SSE events to method status updates
                } catch (err) {
                    addLog({
                        time: new Date().toLocaleTimeString('en', { hour12: false }),
                        type: 'warn',
                        message: 'Malformed log event: ' + e.data,
                    });
                }
            });

            es.onerror = () => {
                es.close();
                esRef.current = null;
                setConnected(false);

                if (unmounted.current) return;

                const delay = RECONNECT_DELAYS[
                    Math.min(retryCount.current, RECONNECT_DELAYS.length - 1)
                ];
                retryCount.current++;

                if (retryCount.current <= RECONNECT_DELAYS.length) {
                    addLog({
                        time: new Date().toLocaleTimeString('en', { hour12: false }),
                        type: 'warn',
                        message: `Log stream disconnected — reconnecting in ${delay / 1000}s `
                            + `(attempt ${retryCount.current}/${RECONNECT_DELAYS.length})`,
                    });
                    setTimeout(connect, delay);
                } else {
                    addLog({
                        time: new Date().toLocaleTimeString('en', { hour12: false }),
                        type: 'error',
                        message: 'Log stream unavailable after 5 attempts. '
                            + 'Check that Go server is running on :8080.',
                    });
                }
            };
        }

        connect();

        return () => {
            unmounted.current = true;
            if (esRef.current) {
                esRef.current.close();
                esRef.current = null;
            }
            setConnected(false);
        };
    }, []); // empty deps — connect once on mount, cleanup on unmount
}
