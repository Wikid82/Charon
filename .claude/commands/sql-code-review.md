# SQL Code Review

Perform a thorough SQL code review of the provided SQL/GORM code focusing on security, performance, maintainability, and database best practices.

**Code to review**: $ARGUMENTS (or selected code / current file if not specified)

## Security Analysis

### SQL Injection Prevention
- All user inputs must use parameterized queries — never string concatenation
- Verify GORM's raw query calls use `?` placeholders or named args, not `fmt.Sprintf`
- Review access controls and principle of least privilege
- Check for sensitive data exposure (avoid `SELECT *` on tables with sensitive columns)

### Access Control & Data Protection
- Role-based access: use database roles instead of direct user permissions
- Sensitive operations are audit-logged
- Encrypted storage for sensitive data (passwords, tokens)

## Performance Optimization

### Query Structure
- Avoid `SELECT *` — use explicit column lists
- Use appropriate JOIN types (INNER vs LEFT vs EXISTS)
- Avoid functions in WHERE clauses that prevent index usage (e.g., `YEAR(date_col)`)
- Use range conditions instead: `date_col >= '2024-01-01' AND date_col < '2025-01-01'`

### Index Strategy
- Identify columns needing indexes (frequently queried in WHERE, JOIN, ORDER BY)
- Composite indexes: correct column order matters
- Avoid over-indexing (impacts INSERT/UPDATE performance)

### Common Anti-Patterns to Flag

```sql
-- N+1 query problem: loop + individual queries → fix with JOIN
-- Correlated subqueries → replace with window functions or JOIN
-- DISTINCT masking join issues → fix the JOIN instead
-- OFFSET pagination on large tables → use cursor-based pagination
-- OR conditions preventing index use → consider UNION ALL
```

## Code Quality

- Consistent naming conventions (snake_case for columns/tables)
- No reserved words as identifiers
- Appropriate data types (don't use TEXT for fixed-length values)
- Constraints enforce data integrity (NOT NULL, FK, CHECK, DEFAULT)

## Output Format

For each issue found:

```
## [PRIORITY] [CATEGORY]: [Brief Description]

**Location**: [Table/line/function]
**Issue**: [Detailed explanation]
**Security Risk**: [If applicable]
**Performance Impact**: [If applicable]
**Recommendation**: [Specific fix with code example]

Before:
[problematic SQL]

After:
[improved SQL]
```

### Summary Assessment
- **Security Score**: [1-10]
- **Performance Score**: [1-10]
- **Maintainability Score**: [1-10]

### Top 3 Priority Actions
1. [Critical fix]
2. [Performance improvement]
3. [Code quality improvement]
