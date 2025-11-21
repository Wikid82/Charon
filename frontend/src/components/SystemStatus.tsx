import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { checkUpdates } from '../api/system';
import { ExternalLink, CheckCircle, AlertCircle } from 'lucide-react';

const SystemStatus: React.FC = () => {
  const [showUpToDate, setShowUpToDate] = useState(true);
  const { data: updateInfo, isLoading } = useQuery({
    queryKey: ['system-updates'],
    queryFn: checkUpdates,
    staleTime: 1000 * 60 * 60, // 1 hour
  });

  useEffect(() => {
    if (updateInfo && !updateInfo.available) {
      const timer = setTimeout(() => {
        setShowUpToDate(false);
      }, 5000); // Hide after 5 seconds
      return () => clearTimeout(timer);
    } else {
      setShowUpToDate(true);
    }
  }, [updateInfo]);

  if (isLoading) return null;

  if (!updateInfo?.available) {
    if (!showUpToDate) return null;
    return (
      <div className="flex items-center text-sm text-green-500 transition-opacity duration-500">
        <CheckCircle className="w-4 h-4 mr-1" />
        <span className="hidden sm:inline">Up to date</span>
      </div>
    );
  }

  return (
    <div className="flex items-center text-sm text-yellow-500 bg-yellow-50 dark:bg-yellow-900/20 px-3 py-1 rounded-full">
      <AlertCircle className="w-4 h-4 mr-2" />
      <span className="mr-2 hidden sm:inline">Update available: {updateInfo.latest_version}</span>
      <a
        href={updateInfo.changelog_url}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center underline hover:text-yellow-600"
      >
        <span className="hidden sm:inline">Changelog</span> <ExternalLink className="w-3 h-3 ml-1" />
      </a>
    </div>
  );
};

export default SystemStatus;
