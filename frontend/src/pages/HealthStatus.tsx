import { useQuery } from '@tanstack/react-query';
import client from '../api/client';

interface HealthResponse {
  status: string;
  service: string;
}

const fetchHealth = async (): Promise<HealthResponse> => {
  const { data } = await client.get<HealthResponse>('/health');
  return data;
};

const HealthStatus = () => {
  const { data, isLoading, isError } = useQuery({ queryKey: ['health'], queryFn: fetchHealth });

  return (
    <section>
      <h2>System Status</h2>
      {isLoading && <p>Checking health…</p>}
      {isError && <p className="error">Unable to reach backend</p>}
      {data && (
        <ul>
          <li>Service: {data.service}</li>
          <li>Status: {data.status}</li>
        </ul>
      )}
    </section>
  );
};

export default HealthStatus;
