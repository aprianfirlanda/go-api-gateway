import { ApiError } from './errors'
import type { ApiRequestOptions } from './types'

const DEFAULT_BASE_URL = 'http://localhost:8080'
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? DEFAULT_BASE_URL

export async function apiRequest<T>(
  path: string,
  options: ApiRequestOptions = {},
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers ?? {}),
    },
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
    signal: options.signal,
  })

  if (!response.ok) {
    let payload: { message?: string; code?: string; details?: unknown } | null
    try {
      payload = (await response.json()) as {
        message?: string
        code?: string
        details?: unknown
      }
    } catch {
      payload = null
    }

    throw new ApiError(
      payload?.message ?? `Request failed with status ${response.status}`,
      response.status,
      payload?.code,
      payload?.details,
    )
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}
