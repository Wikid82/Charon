import client from './client';

export interface LogFile {
  name: string;
  size: number;
  mod_time: string;
}

export interface LogContent {
  lines: string[];
}

export const getLogs = async (): Promise<LogFile[]> => {
  const response = await client.get<LogFile[]>('/logs');
  return response.data;
};

export const getLogContent = async (filename: string, lines: number = 100): Promise<LogContent> => {
  const response = await client.get<LogContent>(`/logs/${filename}?lines=${lines}`);
  return response.data;
};
