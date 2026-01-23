import {ProjectsResponse} from "./types"
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:820';


export class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'APIError';
  }
}

export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });

  const data = await response.json();

  if (!response.ok) {
    throw new APIError(response.status, data.error || 'Request failed');
  }

  return data;
}

export const authAPI = {
  login: async (username: string, password: string) => {
    return apiRequest<{ user_id: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
  },

  signup: async (
    username: string,
    email: string,
    password: string,
    token: string,
    config_path: string
  ) => {
    return apiRequest<{ user_id: string }>('/api/auth/signup', {
      method: 'POST',
      body: JSON.stringify({ username, email, password, token, config_path }),
    });
  },
};

export const projectAPI = {
  createProject: async (
    userId: string,
    projectName: string,
    deploymentType: string,
    contextName: string
  ) => {
    return apiRequest('/api/projects', {
      method: 'POST',
      body: JSON.stringify({
        user_id: userId,
        project_name: projectName,
        deployment_type: deploymentType,
        context_name: contextName,
      }),
    });
  },
  getProjects: async (userId: string): Promise<ProjectsResponse> => {
    return apiRequest(`/api/projects/${userId}`, {
      method: 'GET',
    });
  },
};
