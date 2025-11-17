import { FormEvent, useState } from 'react';
import { useCreateProxyHost, useProxyHosts } from '../hooks/useProxyHosts';

const ProxyHosts = () => {
  const { data, isLoading } = useProxyHosts();
  const mutation = useCreateProxyHost();
  const [formData, setFormData] = useState({
    name: '',
    domain: '',
    target_scheme: 'http',
    target_host: '',
    target_port: 80
  });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    mutation.mutate(formData, {
      onSuccess: () =>
        setFormData({ name: '', domain: '', target_scheme: 'http', target_host: '', target_port: 80 })
    });
  };

  return (
    <section>
      <h2>Proxy Hosts</h2>

      <form onSubmit={handleSubmit}>
        <label>
          Name
          <input
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            required
          />
        </label>
        <label>
          Domain
          <input
            value={formData.domain}
            onChange={(e) => setFormData({ ...formData, domain: e.target.value })}
            required
          />
        </label>
        <label>
          Target Host
          <input
            value={formData.target_host}
            onChange={(e) => setFormData({ ...formData, target_host: e.target.value })}
            required
          />
        </label>
        <label>
          Target Port
          <input
            type="number"
            value={formData.target_port}
            onChange={(e) =>
              setFormData({ ...formData, target_port: Number(e.target.value) || formData.target_port })
            }
            required
          />
        </label>
        <label>
          Scheme
          <select
            value={formData.target_scheme}
            onChange={(e) => setFormData({ ...formData, target_scheme: e.target.value })}
          >
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
          </select>
        </label>
        <button type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? 'Saving...' : 'Add Proxy Host'}
        </button>
        {mutation.isError && <p className="error">Failed to create proxy host</p>}
      </form>

      {isLoading ? (
        <p>Loading hosts…</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Domain</th>
              <th>Target</th>
            </tr>
          </thead>
          <tbody>
            {data?.map((host) => (
              <tr key={host.uuid}>
                <td>{host.name}</td>
                <td>{host.domain}</td>
                <td>
                  {host.target_scheme}://{host.target_host}:{host.target_port}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
};

export default ProxyHosts;
