# Feature Implementation Plan

Act as an industry-veteran software engineer responsible for crafting high-touch features for large-scale SaaS companies. Create a detailed technical implementation plan for: **$ARGUMENTS**

**Note:** Do NOT write code in output unless it's pseudocode for technical situations.

## Output

Save the plan to `docs/plans/current_spec.md`.

## Implementation Plan Structure

For the feature:

### Goal

Feature goal described (3-5 sentences)

### Requirements

- Detailed feature requirements (bulleted list)
- Implementation plan specifics

### Technical Considerations

#### System Architecture Overview

Create a Mermaid architecture diagram showing how this feature integrates into the overall system, including:

- **Frontend Layer**: UI components, state management, client-side logic
- **API Layer**: Gin endpoints, authentication middleware, input validation
- **Business Logic Layer**: Service classes, business rules, workflow orchestration
- **Data Layer**: GORM interactions, caching, external API integrations
- **Infrastructure Layer**: Docker containers, background services, deployment

Show data flow between layers with labeled arrows indicating request/response patterns and event flows.

**Technology Stack Selection**: Document choice rationale for each layer
**Integration Points**: Define clear boundaries and communication protocols
**Deployment Architecture**: Docker containerization strategy

#### Database Schema Design

Mermaid ER diagram showing:
- **Table Specifications**: Detailed field definitions with types and constraints
- **Indexing Strategy**: Performance-critical indexes and rationale
- **Foreign Key Relationships**: Data integrity and referential constraints
- **Migration Strategy**: Version control and deployment approach

#### API Design

- Gin endpoints with full specifications
- Request/response formats with Go struct types
- Authentication/authorization middleware
- Error handling strategies and status codes

#### Frontend Architecture

Component hierarchy using shadcn/ui:
- Layout structure (ASCII tree diagram)
- State flow diagram (Mermaid)
- TanStack Query hooks
- TypeScript interfaces and types

#### Security & Performance

- Authentication/authorization requirements
- Data validation and sanitisation
- Performance optimisation strategies
- OWASP Top 10 compliance

## Implementation Phases

Break down into these phases:

1. **Phase 1**: Playwright E2E Tests (how the feature should behave per UI/UX spec)
2. **Phase 2**: Backend Implementation (Go/Gin/GORM)
3. **Phase 3**: Frontend Implementation (React/TypeScript)
4. **Phase 4**: Integration and Testing
5. **Phase 5**: Documentation and Deployment

## Commit Slicing Strategy

Decide: single PR or multiple PRs. When splitting:
- Ordered PR slices (PR-1, PR-2, ...) with scope, files, dependencies, and validation gates
- Each slice must be independently deployable and testable
- Rollback notes per slice
