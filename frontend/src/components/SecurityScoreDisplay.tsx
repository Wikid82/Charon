import { Shield, ChevronDown, ChevronRight, AlertCircle } from 'lucide-react';
import { useState } from 'react';

import { Badge } from './ui/Badge';
import { Card } from './ui/Card';
import { Progress } from './ui/Progress';

interface SecurityScoreDisplayProps {
  score: number;
  maxScore?: number;
  breakdown?: Record<string, number>;
  suggestions?: string[];
  size?: 'sm' | 'md' | 'lg';
  showDetails?: boolean;
}

const CATEGORY_LABELS: Record<string, string> = {
  hsts: 'HSTS',
  csp: 'Content Security Policy',
  x_frame_options: 'X-Frame-Options',
  x_content_type_options: 'X-Content-Type-Options',
  referrer_policy: 'Referrer Policy',
  permissions_policy: 'Permissions Policy',
  cross_origin: 'Cross-Origin Headers',
};

const CATEGORY_DESCRIPTIONS: Record<string, string> = {
  hsts: 'HTTP Strict Transport Security enforces HTTPS connections',
  csp: 'Content Security Policy prevents XSS and injection attacks',
  x_frame_options: 'Prevents clickjacking by controlling iframe embedding',
  x_content_type_options: 'Prevents MIME type sniffing attacks',
  referrer_policy: 'Controls referrer information sent with requests',
  permissions_policy: 'Restricts browser features and APIs',
  cross_origin: 'Cross-Origin isolation headers for enhanced security',
};

export function SecurityScoreDisplay({
  score,
  maxScore = 100,
  breakdown = {},
  suggestions = [],
  size = 'md',
  showDetails = true,
}: SecurityScoreDisplayProps) {
  const [expandedBreakdown, setExpandedBreakdown] = useState(false);
  const [expandedSuggestions, setExpandedSuggestions] = useState(false);

  const percentage = Math.round((score / maxScore) * 100);

  const getScoreColor = () => {
    if (percentage >= 75) return 'text-green-600 dark:text-green-400';
    if (percentage >= 50) return 'text-yellow-600 dark:text-yellow-400';
    return 'text-red-600 dark:text-red-400';
  };

  const getScoreBgColor = () => {
    if (percentage >= 75) return 'bg-green-100 dark:bg-green-900/20';
    if (percentage >= 50) return 'bg-yellow-100 dark:bg-yellow-900/20';
    return 'bg-red-100 dark:bg-red-900/20';
  };

  const getScoreVariant = (): 'success' | 'warning' | 'error' => {
    if (percentage >= 75) return 'success';
    if (percentage >= 50) return 'warning';
    return 'error';
  };

  const sizeClasses = {
    sm: 'w-12 h-12 text-sm',
    md: 'w-20 h-20 text-2xl',
    lg: 'w-32 h-32 text-4xl',
  };

  if (size === 'sm') {
    return (
      <div className="flex items-center gap-2">
        <div
          className={`${sizeClasses[size]} rounded-full ${getScoreBgColor()} flex items-center justify-center font-bold ${getScoreColor()}`}
        >
          {score}
        </div>
        <div className="text-xs text-gray-600 dark:text-gray-400">/ {maxScore}</div>
      </div>
    );
  }

  return (
    <Card className="p-4">
      <div className="flex items-start gap-4">
        {/* Circular Score Display */}
        <div className="shrink-0">
          <div
            className={`${sizeClasses[size]} rounded-full ${getScoreBgColor()} flex flex-col items-center justify-center font-bold ${getScoreColor()}`}
          >
            <div className="flex items-baseline">
              <span>{score}</span>
              <span className="text-sm opacity-75">/{maxScore}</span>
            </div>
            <div className="text-xs font-normal opacity-75">Security</div>
          </div>
        </div>

        {/* Score Info */}
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-2">
            <Shield className="w-5 h-5 text-gray-400" />
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Security Score</h3>
            <Badge variant={getScoreVariant()}>{percentage}%</Badge>
          </div>

          {/* Overall Progress Bar */}
          <Progress value={percentage} variant={getScoreVariant()} className="mb-4" />

          {showDetails && (
            <>
              {/* Breakdown Section */}
              {Object.keys(breakdown).length > 0 && (
                <div className="mt-4">
                  <button
                    onClick={() => setExpandedBreakdown(!expandedBreakdown)}
                    className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-colors"
                  >
                    {expandedBreakdown ? (
                      <ChevronDown className="w-4 h-4" />
                    ) : (
                      <ChevronRight className="w-4 h-4" />
                    )}
                    Score Breakdown by Category
                  </button>

                  {expandedBreakdown && (
                    <div className="mt-3 space-y-3 pl-6">
                      {Object.entries(breakdown).map(([category, categoryScore]) => {
                        const categoryMax = getCategoryMax(category);
                        const categoryPercent = Math.round((categoryScore / categoryMax) * 100);

                        return (
                          <div key={category} className="space-y-1">
                            <div className="flex items-center justify-between text-sm">
                              <span
                                className="text-gray-700 dark:text-gray-300"
                                title={CATEGORY_DESCRIPTIONS[category]}
                              >
                                {CATEGORY_LABELS[category] || category}
                              </span>
                              <span className="text-gray-600 dark:text-gray-400 font-mono">
                                {categoryScore}/{categoryMax}
                              </span>
                            </div>
                            <Progress
                              value={categoryPercent}
                              variant={categoryPercent >= 70 ? 'success' : categoryPercent >= 40 ? 'warning' : 'error'}
                            />
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}

              {/* Suggestions Section */}
              {suggestions.length > 0 && (
                <div className="mt-4">
                  <button
                    onClick={() => setExpandedSuggestions(!expandedSuggestions)}
                    className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-colors"
                  >
                    {expandedSuggestions ? (
                      <ChevronDown className="w-4 h-4" />
                    ) : (
                      <ChevronRight className="w-4 h-4" />
                    )}
                    Security Suggestions ({suggestions.length})
                  </button>

                  {expandedSuggestions && (
                    <ul className="mt-3 space-y-2 pl-6">
                      {suggestions.map((suggestion, index) => (
                        <li key={index} className="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400">
                          <AlertCircle className="w-4 h-4 text-yellow-500 shrink-0 mt-0.5" />
                          <span>{suggestion}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

// Helper function to determine max score for each category
function getCategoryMax(category: string): number {
  const maxScores: Record<string, number> = {
    hsts: 25,
    csp: 25,
    x_frame_options: 10,
    x_content_type_options: 10,
    referrer_policy: 10,
    permissions_policy: 10,
    cross_origin: 10,
  };

  return maxScores[category] || 10;
}
