package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// reader acumula los errores de configuración para reportarlos todos juntos: corregir el .env de una sola vez.
type reader struct {
	getenv func(string) string
	errs   []error
}

func (r *reader) fail(msg string) { r.errs = append(r.errs, errors.New(msg)) }

func (r *reader) required(key string) string {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		r.fail(key + " es obligatoria")
	}
	return v
}

func (r *reader) optional(key, def string) string {
	if v := strings.TrimSpace(r.getenv(key)); v != "" {
		return v
	}
	return def
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key + " no es una duración válida (ej. 5s)")
		return def
	}
	return d
}

func (r *reader) intInRange(key string, def, lo, hi int) int {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		r.fail(fmt.Sprintf("%s debe ser un entero entre %d y %d", key, lo, hi))
		return def
	}
	return n
}

func (r *reader) validURL(key, v string) {
	if v == "" {
		return
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		r.fail(key + " no es una URL http(s) válida")
	}
}
