package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type contextKey string

const AuthContextKey contextKey = "auth"

type AuthClaims struct {
	UserID    string
	CompanyID string
	Roles     []string
	Perms     []string
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if claims, ok := ctx.Value(AuthContextKey).(*AuthClaims); ok {
		return claims.UserID
	}
	return "system"
}

// GetCompanyID extracts company ID from context.
func GetCompanyID(ctx context.Context) string {
	if claims, ok := ctx.Value(AuthContextKey).(*AuthClaims); ok {
		return claims.CompanyID
	}
	return ""
}

// GetID attempts to extract an "ID" field from any struct or pointer to struct.
func GetID(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Sprint(v)
	}
	f := rv.FieldByName("ID")
	if f.IsValid() && f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}

// GetDedupeKey attempts to find a suitable unique key for idempotency in a request struct.
func GetDedupeKey(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range []string{"DedupeKey", "DedupeID", "ID", "RequestID"} {
		f := rv.FieldByName(name)
		if f.IsValid() && f.Kind() == reflect.String && f.String() != "" {
			return f.String()
		}
	}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		fieldName := rv.Type().Field(i).Name
		if len(fieldName) > 2 && fieldName[len(fieldName)-2:] == "ID" {
			if f.Kind() == reflect.String && f.String() != "" {
				return f.String()
			}
		}
	}
	return ""
}

// Marshal encodes an object to JSON string.
func Marshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Unmarshal decodes JSON bytes to an object.
func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// CopyNonEmptyFields copies non-zero fields from src to dst using reflection.
func CopyNonEmptyFields(src, dst interface{}) {
	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst)
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}
	if !dstVal.CanSet() {
		return
	}
	srcType := srcVal.Type()
	for i := 0; i < srcVal.NumField(); i++ {
		fieldName := srcType.Field(i).Name
		if fieldName == "ID" || fieldName == "CreatedAt" || fieldName == "DeletedAt" {
			continue
		}
		srcField := srcVal.Field(i)
		dstField := dstVal.FieldByName(fieldName)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}
		if srcField.IsZero() {
			continue
		}
		if dstField.Type() == srcField.Type() {
			dstField.Set(srcField)
		}
	}
}

// IsZero reports whether value is a Go zero value.
func IsZero(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}

var validate = validator.New()

// Validate checks a struct for validation tags.
func Validate(s interface{}) error {
	return validate.Struct(s)
}

// _ is a compile guard to ensure strings is used.
var _ = strings.ToLower
