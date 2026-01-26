# Database Migrations

This document tracks database schema changes and migration notes for the Charon project.

## Migration Strategy

Charon uses GORM's AutoMigrate feature for database schema management. Migrations are automatically applied when the application starts. The migrations are defined in:

- Main application: `backend/cmd/api/main.go` (security tables)
- Route registration: `backend/internal/api/routes/routes.go` (all other tables)

## Migration History

### 2024-12-XX: DNSProvider KeyVersion Field Addition

**Purpose**: Added encryption key rotation support for DNS provider credentials.

**Changes**:

- Added `KeyVersion` field to `DNSProvider` model
  - Type: `int`
  - GORM tags: `gorm:"default:1;index"`
  - JSON tag: `json:"key_version"`
  - Purpose: Tracks which encryption key version was used for credentials

**Backward Compatibility**:

- Existing records will automatically get `key_version = 1` (GORM default)
- No data migration required
- The field is indexed for efficient queries during key rotation operations
- Compatible with both basic encryption and rotation service

**Migration Execution**:

```go
// Automatically handled by GORM AutoMigrate in routes.go:
db.AutoMigrate(&models.DNSProvider{})
```

**Related Files**:

- `backend/internal/models/dns_provider.go` - Model definition
- `backend/internal/crypto/rotation_service.go` - Key rotation logic
- `backend/internal/services/dns_provider_service.go` - Service implementation

**Testing**:

- All existing tests pass with the new field
- Test database initialization updated to use shared cache mode
- No breaking changes to existing functionality

**Security Notes**:

- The `KeyVersion` field is essential for secure key rotation
- It allows re-encrypting credentials with new keys while maintaining access to old data
- The rotation service can decrypt using any registered key version
- New records always use version 1 unless explicitly rotated

---

## Best Practices for Future Migrations

### Adding New Fields

1. **Always include GORM tags**:

   ```go
   FieldName string `json:"field_name" gorm:"default:value;index"`
   ```

2. **Set appropriate defaults** to ensure backward compatibility

3. **Add indexes** for fields used in queries or joins

4. **Document** the migration in this README

### Testing Migrations

1. **Test with clean database**: Verify AutoMigrate creates tables correctly

2. **Test with existing database**: Verify new fields are added without data loss

3. **Update test setup**: Ensure test databases include all new tables/fields

### Common Issues and Solutions

#### "no such table" Errors in Tests

**Problem**: Tests fail with "no such table: table_name" errors

**Solutions**:

1. Ensure AutoMigrate is called in test setup:

   ```go
   db.AutoMigrate(&models.YourModel{})
   ```

2. For parallel tests, use shared cache mode:

   ```go
   db, _ := gorm.Open(sqlite.Open(":memory:?cache=shared&mode=memory&_mutex=full"), &gorm.Config{})
   ```

3. Verify table exists after migration:

   ```go
   if !db.Migrator().HasTable(&models.YourModel{}) {
       t.Fatal("failed to create table")
   }
   ```

#### Migration Order Matters

**Problem**: Foreign key constraints fail during migration

**Solution**: Migrate parent tables before child tables:

```go
db.AutoMigrate(
    &models.Parent{},
    &models.Child{},  // References Parent
)
```

#### Concurrent Test Access

**Problem**: Tests interfere with each other's database access

**Solution**: Configure connection pooling for SQLite:

```go
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(1)
sqlDB.SetMaxIdleConns(1)
```

---

## Rollback Strategy

Since Charon uses AutoMigrate, which only adds columns (never removes), rollback requires:

1. **Code rollback**: Deploy previous version
2. **Manual cleanup** (if needed): Drop added columns via SQL
3. **Data preservation**: Old columns remain, data is safe

**Note**: Always test migrations in a development environment first.

---

## See Also

- [GORM Migration Documentation](https://gorm.io/docs/migration.html)
- [SQLite Best Practices](https://www.sqlite.org/bestpractice.html)
- Project testing guidelines: `/.github/instructions/testing.instructions.md`
