# Professional Prompt Builder

Guide me through creating a new Claude Code command (`.claude/commands/*.md`) or agent (`.claude/agents/*.md`) by systematically gathering requirements, then generating a complete, production-ready file.

**What to build**: $ARGUMENTS (or describe what you want if not specified)

## Discovery Process

I will ask targeted questions across these areas. Answer each section, then I'll generate the complete file.

### 1. Identity & Purpose
- What is the intended filename? (e.g., `generate-react-component.md`)
- Is this a **command** (slash command invoked by user) or an **agent** (autonomous subagent)?
- One-sentence description of what it accomplishes
- Category: code generation / analysis / documentation / testing / refactoring / architecture / security

### 2. Persona Definition
- What role/expertise should the AI embody?
- Example: "Senior Go engineer with 10+ years in security-sensitive API design"

### 3. Task Specification
- Primary task (explicit and measurable)
- Secondary/optional tasks
- What does the user provide as input? (`$ARGUMENTS`, selected code, file reference)
- Constraints that must be followed

### 4. Context Requirements
- Does it use `$ARGUMENTS` for user input?
- Does it reference specific files in the codebase?
- Does it need to read/write specific directories?

### 5. Instructions & Standards
- Step-by-step process to follow
- Specific coding standards, frameworks, or libraries
- Patterns to enforce, things to avoid
- Reference any existing `.github/instructions/` files?

### 6. Output Requirements
- Format: code / markdown / structured report / file creation
- If creating files: where and what naming convention?
- Examples of ideal output (for few-shot learning)

### 7. Quality & Validation
- How is success measured?
- What validation steps to include?
- Common failure modes to address?

## Template Generation

After gathering requirements, I will generate the complete file:

**For commands** (`.claude/commands/`):
```md
# [Command Title]

[Persona definition]

**Input**: $ARGUMENTS

## [Instructions Section]

[Step-by-step instructions]

## [Output Format]

[Expected structure]

## Constraints

- [Constraint 1]
```

**For agents** (`.claude/agents/`):
```md
---
name: agent-name
description: [Routing description — how Claude Code decides to use this agent]
---

[System prompt with persona, workflow, constraints]
```

Please start by answering section 1 (Identity & Purpose). I'll guide you through each section systematically.
