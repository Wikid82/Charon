import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import client from '../api/client';

export interface ProxyHost {
  id: number;
  uuid: string;
  name: string;
  domain: string;
  target_scheme: string;
  target_host: string;
  target_port: number;
  enable_tls: boolean;
  enable_websockets: boolean;
}

const fetchProxyHosts = async (): Promise<ProxyHost[]> => {
  const { data } = await client.get<ProxyHost[]>('/proxy-hosts');
  return data;
};

export const useProxyHosts = () => {
  return useQuery({ queryKey: ['proxy-hosts'], queryFn: fetchProxyHosts });
};

export const useCreateProxyHost = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: Partial<ProxyHost>) => {
      const { data } = await client.post<ProxyHost>('/proxy-hosts', payload);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
    }
  });
};
