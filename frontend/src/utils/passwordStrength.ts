export interface PasswordStrength {
  score: number; // 0-4
  label: string;
  color: string; // Tailwind color class prefix (e.g., 'red', 'yellow', 'green')
  feedback: string[];
}

export function calculatePasswordStrength(password: string): PasswordStrength {
  let score = 0;
  const feedback: string[] = [];

  if (!password) {
    return {
      score: 0,
      label: 'Empty',
      color: 'gray',
      feedback: [],
    };
  }

  // Length check
  if (password.length < 8) {
    feedback.push('Too short (min 8 chars)');
  } else {
    score += 1;
  }

  if (password.length >= 12) {
    score += 1;
  }

  // Complexity checks
  const hasLower = /[a-z]/.test(password);
  const hasUpper = /[A-Z]/.test(password);
  const hasNumber = /\d/.test(password);
  const hasSpecial = /[^A-Za-z0-9]/.test(password);

  const varietyCount = [hasLower, hasUpper, hasNumber, hasSpecial].filter(Boolean).length;

  if (varietyCount >= 3) {
    score += 1;
  }
  if (varietyCount === 4) {
    score += 1;
  }

  // Penalties
  if (varietyCount < 2 && password.length >= 8) {
    feedback.push('Add more variety (uppercase, numbers, symbols)');
  }

  // Cap score at 4
  score = Math.min(score, 4);

  // Determine label and color
  let label = 'Very Weak';
  let color = 'red';

  switch (score) {
    case 0:
    case 1:
      label = 'Weak';
      color = 'red';
      break;
    case 2:
      label = 'Fair';
      color = 'yellow';
      break;
    case 3:
      label = 'Good';
      color = 'green';
      break;
    case 4:
      label = 'Strong';
      color = 'green';
      break;
  }

  return { score, label, color, feedback };
}
