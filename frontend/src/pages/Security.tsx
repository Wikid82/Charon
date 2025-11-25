import ForwardAuthSettings from '../components/ForwardAuthSettings';

export default function Security() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Security</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Manage security settings and authentication providers.
        </p>
      </div>

      <ForwardAuthSettings />
    </div>
  );
}
