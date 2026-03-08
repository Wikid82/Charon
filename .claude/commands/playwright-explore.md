# Playwright: Explore Website

Explore a website to identify key functionalities for testing purposes.

**URL to explore**: $ARGUMENTS (if not provided, ask the user)

## Instructions

1. Navigate to the provided URL using Playwright
2. Identify and interact with 3–5 core features or user flows
3. Document:
   - User interactions performed
   - Relevant UI elements and their accessible locators (`getByRole`, `getByLabel`, `getByText`)
   - Expected outcomes for each interaction
4. Close the browser context upon completion
5. Provide a concise summary of findings
6. Propose and generate test cases based on the exploration

## Output Format

```markdown
## Exploration Summary

**URL**: [URL explored]
**Date**: [Date]

## Core Features Identified

### Feature 1: [Name]
- **Description**: [What it does]
- **User Flow**: [Steps taken]
- **Key Elements**: [Locators found]
- **Expected Outcome**: [What should happen]

### Feature 2: [Name]
...

## Proposed Test Cases

1. **[Test Name]**: [Scenario and expected outcome]
2. **[Test Name]**: [Scenario and expected outcome]
...
```

## Notes

- Use role-based locators wherever possible (`getByRole`, `getByLabel`, `getByText`)
- Note any accessibility issues encountered during exploration
- For the Charon dev environment, the default URL is `http://localhost:8080`
- Ensure the dev environment is running first: `/docker-start-dev`
