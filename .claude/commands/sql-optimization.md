# SQL Performance Optimization

Expert SQL performance optimization for the provided query or codebase.

**Query/code to optimize**: $ARGUMENTS (or selected code / current file if not specified)

## Core Optimization Areas

### 1. Query Performance Analysis

Common patterns to fix:

```sql
-- BAD: Function in WHERE prevents index use
WHERE YEAR(created_at) = 2024
-- GOOD: Range condition
WHERE created_at >= '2024-01-01' AND created_at < '2025-01-01'

-- BAD: Correlated subquery (runs once per row)
WHERE price > (SELECT AVG(price) FROM products p2 WHERE p2.category_id = p.category_id)
-- GOOD: Window function
SELECT *, AVG(price) OVER (PARTITION BY category_id) FROM products

-- BAD: SELECT *
SELECT * FROM large_table JOIN another_table ON ...
-- GOOD: Explicit columns
SELECT lt.id, lt.name, at.value FROM ...

-- BAD: OFFSET pagination (slow at large offsets)
SELECT * FROM products ORDER BY created_at DESC LIMIT 20 OFFSET 10000
-- GOOD: Cursor-based pagination
SELECT * FROM products WHERE created_at < ? ORDER BY created_at DESC LIMIT 20

-- BAD: Multiple aggregation queries
SELECT COUNT(*) FROM orders WHERE status = 'pending';
SELECT COUNT(*) FROM orders WHERE status = 'shipped';
-- GOOD: Single conditional aggregation
SELECT COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending,
       COUNT(CASE WHEN status = 'shipped' THEN 1 END) as shipped
FROM orders;
```

### 2. Index Strategy

- **Missing indexes**: Identify unindexed columns in WHERE, JOIN ON, ORDER BY
- **Composite index column order**: Most selective column first (unless query pattern dictates otherwise)
- **Covering indexes**: Include all columns needed to satisfy query without table lookup
- **Partial indexes**: Index only rows matching a WHERE condition (e.g., `WHERE status = 'active'`)
- **Over-indexing**: Remove unused indexes (every index slows INSERT/UPDATE/DELETE)

### 3. JOIN Optimization

- Filter early using INNER JOIN instead of LEFT JOIN + WHERE IS NOT NULL
- Smallest result set as the driving table
- Eliminate Cartesian products (missing join conditions)
- Use EXISTS over IN for subqueries when checking for existence

### 4. Batch Operations

```sql
-- BAD: Row-by-row inserts
INSERT INTO products (name) VALUES ('A');
INSERT INTO products (name) VALUES ('B');
-- GOOD: Batch insert
INSERT INTO products (name) VALUES ('A'), ('B'), ('C');
```

## GORM-Specific Notes (Go)

- Use `db.Select([]string{"id", "name"})` — never `db.Find(&result)` on large tables
- Use `db.Where("status = ?", status)` — parameterized always
- For complex aggregations, prefer raw SQL with `db.Raw()` + named args
- Use `db.FindInBatches()` for large dataset iteration

## Output Format

For each optimization:
1. **Problem**: What's slow/inefficient and why
2. **Before**: Current code
3. **After**: Optimized code
4. **Index recommendation**: SQL CREATE INDEX statement if needed
5. **Expected improvement**: Estimated performance gain

## Optimization Methodology

1. **Identify**: Slowest queries by execution time or call frequency
2. **Analyze**: Check execution plans (use `EXPLAIN` / `EXPLAIN ANALYZE`)
3. **Optimize**: Apply appropriate technique
4. **Test**: Verify improvement with realistic data volumes
5. **Monitor**: Track performance metrics over time
