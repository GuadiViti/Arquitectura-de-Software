import { request, type ApiResponse } from './client';

export type ServiceState = 'ready' | 'not_ready' | 'unreachable';

export interface ServiceStatus {
  name: string;
  status: ServiceState;
  latency_ms: number;
  checks: Record<string, string>;
}

export interface SystemStatus {
  status: 'ready' | 'degraded';
  checked_at: string;
  services: ServiceStatus[];
}

/** GET /api/v1/status. 503 también trae el resumen (sistema degradado). */
export function getSystemStatus(): Promise<ApiResponse<SystemStatus>> {
  return request<SystemStatus>('/api/v1/status', { acceptStatuses: [200, 503] });
}
