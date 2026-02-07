package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitAuthPackage генерирует пакет для работы с JWT
func (e *Emitter) EmitAuthPackage(auth *normalizer.AuthDef) error {
	if auth == nil {
		auth = &normalizer.AuthDef{
			Alg: "RS256",
		}
	}
	tmplPath := filepath.Join(e.TemplatesDir, "auth_pkg.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/auth_pkg.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("auth_pkg").Funcs(template.FuncMap{
		"GoModule": func() string { return e.GoModule },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "pkg", "auth")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, auth); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "auth.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Auth Package: %s\n", path)
	return nil
}

// EmitRefreshTokenStorePort генерирует интерфейс для хранилища рефреш-токенов
func (e *Emitter) EmitRefreshTokenStorePort() error {
	tmplPath := filepath.Join(e.TemplatesDir, "refresh_store.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/refresh_store.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("refresh_store").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "refreshstore.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Refresh Store Port: %s\n", path)
	return nil
}

// EmitRefreshTokenStoreMemory генерирует ин-мемори реализацию хранилища токенов
func (e *Emitter) EmitRefreshTokenStoreMemory() error {
	tmplPath := filepath.Join(e.TemplatesDir, "refresh_store_memory.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/refresh_store_memory.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("refresh_store_memory").Funcs(template.FuncMap{"GoModule": func() string { return e.GoModule }}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "auth", "memory")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "store.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Memory Refresh Store: %s\n", path)
	return nil
}

// EmitRefreshTokenStoreRedis генерирует Redis реализацию хранилища токенов
func (e *Emitter) EmitRefreshTokenStoreRedis() error {
	tmplPath := filepath.Join(e.TemplatesDir, "refresh_store_redis.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/refresh_store_redis.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("refresh_store_redis").Funcs(template.FuncMap{"GoModule": func() string { return e.GoModule }}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "auth", "redis")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "store.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Redis Refresh Store: %s\n", path)
	return nil
}

// EmitRefreshTokenStorePostgres генерирует Postgres реализацию хранилища токенов
func (e *Emitter) EmitRefreshTokenStorePostgres() error {
	tmplPath := filepath.Join(e.TemplatesDir, "refresh_store_postgres.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/refresh_store_postgres.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("refresh_store_postgres").Funcs(template.FuncMap{"GoModule": func() string { return e.GoModule }}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "auth", "postgres")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "store.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Postgres Refresh Store: %s\n", path)
	return nil
}

// EmitRefreshTokenStoreHybrid генерирует Hybrid реализацию хранилища токенов
func (e *Emitter) EmitRefreshTokenStoreHybrid() error {
	tmplPath := filepath.Join(e.TemplatesDir, "refresh_store_hybrid.tmpl")
	if _, err := os.Stat(tmplPath); err != nil {
		tmplPath = "templates/refresh_store_hybrid.tmpl"
	}
	
tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("refresh_store_hybrid").Funcs(template.FuncMap{"GoModule": func() string { return e.GoModule }}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "adapter", "auth", "hybrid")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "store.go")
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Hybrid Refresh Store: %s\n", path)
	return nil
}
