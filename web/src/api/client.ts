import axios, { type AxiosError, type AxiosRequestConfig } from 'axios'

export class ApiError extends Error {
  code: string | number
  status?: number
  data?: any

  constructor(message: string, code: string | number = 'ERROR', status?: number, data?: any) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.data = data
  }
}

let errorNotifier: ((message: string) => void) | null = null

export function setErrorNotifier(fn: (message: string) => void) {
  errorNotifier = fn
}

function notify(message: string) {
  if (errorNotifier) errorNotifier(message)
}

const client = axios.create({
  baseURL: '/api',
  timeout: 120000,
  withCredentials: true,
  headers: { 'X-Requested-With': 'XMLHttpRequest' },
})

client.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob') {
      return response.data
    }
    const payload = response.data
    if (payload && typeof payload === 'object' && 'code' in payload) {
      if (payload.code === 0) {
        return payload.data
      }
      const message = payload.message || '请求失败'
      if (!response.config.silent) notify(message)
      return Promise.reject(new ApiError(message, payload.code, response.status, payload.data))
    }
    return payload
  },
  async (error: AxiosError) => {
    const response = error.response
    if (response?.status === 401) {
      const path = window.location.pathname
      if (!path.startsWith('/login') && !path.startsWith('/auth/callback')) {
        const redirect = encodeURIComponent(path + window.location.search)
        window.location.href = `/login?redirect=${redirect}`
      }
      return Promise.reject(new ApiError('登录已失效，请重新登录', 'UNAUTHORIZED', 401))
    }
    let payload: any = response?.data
    if (payload instanceof Blob) {
      try {
        payload = JSON.parse(await payload.text())
      } catch {
        payload = null
      }
    }
    const message = payload?.message || (error.code === 'ECONNABORTED' ? '请求超时，请稍后重试' : error.message) || '网络请求失败'
    if (!error.config?.silent) notify(message)
    return Promise.reject(new ApiError(message, payload?.code ?? 'NETWORK', response?.status, payload?.data))
  },
)

export const api = {
  get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    return client.get(url, config) as unknown as Promise<T>
  },
  post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    return client.post(url, data, config) as unknown as Promise<T>
  },
  put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    return client.put(url, data, config) as unknown as Promise<T>
  },
  delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    return client.delete(url, config) as unknown as Promise<T>
  },
  upload<T = any>(url: string, form: FormData, config?: AxiosRequestConfig): Promise<T> {
    return client.post(url, form, { ...config, headers: { 'Content-Type': 'multipart/form-data' } }) as unknown as Promise<T>
  },
  async download(url: string, config?: AxiosRequestConfig): Promise<Blob> {
    const blob = (await client.get(url, { ...config, responseType: 'blob', silent: true })) as unknown as Blob
    if (blob.type.includes('application/json')) {
      try {
        const payload = JSON.parse(await blob.text())
        throw new ApiError(payload.message || '下载失败', payload.code ?? 'DOWNLOAD_FAILED')
      } catch (error) {
        if (error instanceof ApiError) throw error
        throw new ApiError('下载失败')
      }
    }
    return blob
  },
}
