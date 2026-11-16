package services

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// SentDefaultColumns returns the database columns of model's fields that carry a default tag and whose JSON name is one
// of sentJSONKeys (the keys the client sent). When kinds is given, a field is kept only if its Go kind is one of them.
// The schema is read from model, so a defaulted field added later needs no change here.
func SentDefaultColumns(tx *gorm.DB, model any, sentJSONKeys []string, kinds ...reflect.Kind) ([]string, error) {
	stmt := &gorm.Statement{DB: tx}
	if err := stmt.Parse(model); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	var columns []string
	for _, field := range stmt.Schema.Fields {
		if !field.HasDefaultValue || field.DBName == "" {
			continue
		}
		if !slices.Contains(sentJSONKeys, jsonFieldName(field)) {
			continue
		}
		if len(kinds) > 0 && !slices.Contains(kinds, field.FieldType.Kind()) {
			continue
		}
		columns = append(columns, field.DBName)
	}
	return columns, nil
}

// jsonFieldName returns the JSON name of a schema field, or "" when the field is not serialised.
func jsonFieldName(field *schema.Field) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// CreateKeepingSent inserts value and then restores the defaulted columns in sentColumns, so a value the client sent as
// false, "" or 0 is stored as sent. GORM replaces the zero value of a field with a default tag by the default when it
// creates the row, in the SQL and in the struct. So each sent value is read before the insert, and written back with a
// map update after it, and the struct is set to the same values. A map update writes zero values and does not save
// associations. The steps run on tx, which the caller's transaction must own.
func CreateKeepingSent(tx *gorm.DB, value any, sentColumns []string) error {
	ctx := context.Background()
	stmt := &gorm.Statement{DB: tx}
	if err := stmt.Parse(value); err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	rv := reflect.Indirect(reflect.ValueOf(value))
	sent := make(map[string]any, len(sentColumns))
	fields := make(map[string]*schema.Field, len(sentColumns))
	for _, column := range sentColumns {
		field := stmt.Schema.LookUpField(column)
		if field == nil {
			continue
		}
		sentValue, _ := field.ValueOf(ctx, rv)
		sent[column] = sentValue
		fields[column] = field
	}

	if err := tx.Create(value).Error; err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	if len(sent) == 0 {
		return nil
	}
	if err := tx.Model(value).Updates(sent).Error; err != nil {
		return fmt.Errorf("restore sent values: %w", err)
	}
	for column, sentValue := range sent {
		if err := fields[column].Set(ctx, rv, sentValue); err != nil {
			return fmt.Errorf("set sent value %s: %w", column, err)
		}
	}
	return nil
}
