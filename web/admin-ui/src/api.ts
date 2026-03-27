type ApiEnvelope<T> = {
  data?: T;
  error?: string;
};

async function parseResponse<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("content-type") ?? "";

  if (!contentType.includes("application/json")) {
    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }
    return undefined as T;
  }

  const payload = (await response.json()) as ApiEnvelope<T>;
  if (!response.ok || payload.error) {
    throw new Error(payload.error || `Request failed with status ${response.status}`);
  }

  return payload.data as T;
}

export async function apiRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...(init?.headers ?? {})
    }
  });

  return parseResponse<T>(response);
}

export function apiGet<T>(path: string) {
  return apiRequest<T>(path);
}

export function apiPost<T>(path: string, body?: unknown) {
  return apiRequest<T>(path, {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body)
  });
}

export function apiDelete<T>(path: string) {
  return apiRequest<T>(path, {
    method: "DELETE"
  });
}
