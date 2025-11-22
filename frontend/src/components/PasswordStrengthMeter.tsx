import React from 'react';
import { calculatePasswordStrength } from '../utils/passwordStrength';

interface Props {
  password: string;
}

export const PasswordStrengthMeter: React.FC<Props> = ({ password }) => {
  const { score, label, color, feedback } = calculatePasswordStrength(password);

  // Calculate width percentage based on score (0-4)
  // 0: 5%, 1: 25%, 2: 50%, 3: 75%, 4: 100%
  const width = Math.max(5, (score / 4) * 100);

  // Map color name to Tailwind classes
  const getColorClass = (c: string) => {
    switch (c) {
      case 'red': return 'bg-red-500';
      case 'yellow': return 'bg-yellow-500';
      case 'green': return 'bg-green-500';
      default: return 'bg-gray-300';
    }
  };

  const getTextColorClass = (c: string) => {
    switch (c) {
      case 'red': return 'text-red-500';
      case 'yellow': return 'text-yellow-600';
      case 'green': return 'text-green-600';
      default: return 'text-gray-500';
    }
  };

  if (!password) return null;

  return (
    <div className="mt-2 space-y-1">
      <div className="flex justify-between items-center text-xs">
        <span className={`font-medium ${getTextColorClass(color)}`}>
          {label}
        </span>
        {feedback.length > 0 && (
          <span className="text-gray-500 dark:text-gray-400">
            {feedback[0]}
          </span>
        )}
      </div>

      <div className="h-1.5 w-full bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full transition-all duration-300 ease-out ${getColorClass(color)}`}
          style={{ width: `${width}%` }}
        />
      </div>
    </div>
  );
};
