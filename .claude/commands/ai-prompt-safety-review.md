# AI Prompt Engineering Safety Review

Conduct a comprehensive safety, bias, security, and effectiveness analysis of the provided prompt, then generate an improved version.

**Prompt to review**: $ARGUMENTS (or paste the prompt if not provided)

## Analysis Framework

### 1. Safety Assessment
- **Harmful Content Risk**: Could this generate harmful, dangerous, or inappropriate content?
- **Violence & Hate Speech**: Could output promote violence, discrimination, or hate speech?
- **Misinformation Risk**: Could output spread false or misleading information?
- **Illegal Activities**: Could output promote illegal activities or cause personal harm?

### 2. Bias Detection
- **Gender/Racial/Cultural Bias**: Does the prompt assume or reinforce stereotypes?
- **Socioeconomic/Ability Bias**: Are there unexamined assumptions about users?

### 3. Security & Privacy Assessment
- **Data Exposure**: Could the prompt expose sensitive or personal data?
- **Prompt Injection**: Is the prompt vulnerable to injection attacks?
- **Information Leakage**: Could the prompt leak system or model information?
- **Access Control**: Does the prompt respect appropriate access boundaries?

### 4. Effectiveness Evaluation (Score 1–5 each)
- **Clarity**: Is the task clearly stated and unambiguous?
- **Context**: Is sufficient background provided?
- **Constraints**: Are output requirements and limitations defined?
- **Format**: Is the expected output format specified?
- **Specificity**: Specific enough for consistent results?

### 5. Advanced Pattern Analysis
- **Pattern Type**: Zero-shot / Few-shot / Chain-of-thought / Role-based / Hybrid
- **Pattern Effectiveness**: Is the chosen pattern optimal for the task?
- **Context Utilization**: How effectively is context leveraged?

### 6. Technical Robustness
- **Input Validation**: Does it handle edge cases and invalid inputs?
- **Error Handling**: Are potential failure modes considered?
- **Maintainability**: Easy to update and modify?

## Output Format

```markdown
## Prompt Analysis Report

**Original Prompt:** [User's prompt]
**Task Classification:** [Code generation / analysis / documentation / etc.]
**Complexity Level:** [Simple / Moderate / Complex]

## Safety Assessment
- Harmful Content Risk: [Low/Medium/High] — [specific concerns]
- Bias Detection: [None/Minor/Major] — [specific bias types]
- Privacy Risk: [Low/Medium/High]
- Security Vulnerabilities: [None/Minor/Major]

## Effectiveness Evaluation
- Clarity: [Score] — [assessment]
- Context Adequacy: [Score] — [assessment]
- Constraint Definition: [Score] — [assessment]
- Format Specification: [Score] — [assessment]

## Critical Issues Identified
1. [Issue with severity]

## Strengths Identified
1. [Strength]

---

## Improved Prompt

[Complete improved prompt with all enhancements]

### Key Improvements Made
1. Safety Strengthening: [specific improvement]
2. Bias Mitigation: [specific improvement]
3. Security Hardening: [specific improvement]
4. Clarity Enhancement: [specific improvement]

## Testing Recommendations
- [Test case with expected outcome]
- [Edge case with expected outcome]
- [Safety test with expected outcome]
```

## Constraints

- Always prioritise safety over functionality
- Flag any potential risks with specific mitigation strategies
- Consider edge cases and potential misuse scenarios
- Recommend appropriate constraints and guardrails
- Follow responsible AI principles (Microsoft, OpenAI, Google AI guidelines)
