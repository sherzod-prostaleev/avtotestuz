export class ApiError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  const data = await res.json();
  if (!res.ok) {
    const errCode = data?.error?.code ?? "unknown_error";
    const errMessage = data?.error?.message ?? "An error occurred";
    throw new ApiError(errMessage, errCode, res.status);
  }
  return data.data !== undefined ? data.data : data;
}

export async function apiGet<T>(path: string): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  return handleResponse<T>(res);
}

export async function apiPost<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}

export async function apiPatch<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}

export async function apiDelete<T, B = unknown>(path: string, body?: B): Promise<T> {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  const res = await fetch(`/api/proxy/${cleanPath}`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  return handleResponse<T>(res);
}
