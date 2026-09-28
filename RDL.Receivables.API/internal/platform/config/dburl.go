package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Puertos de Supavisor: 6543 modo transacción (la app), 5432 modo sesión (migraciones, necesitan `set role`).
const (
	PoolerTransactionPort = "6543"
	PoolerSessionPort     = "5432"
)

// PoolerURL arma la URL de conexión a Supavisor. El usuario lleva el project ref como sufijo (<rol>.<ref>);
// la contraseña NO va en la URL: se pasa aparte (DB_PASSWORD) para no tener que codificar caracteres especiales.
func PoolerURL(supabaseURL, host, role, port string) (string, error) {
	ref, err := projectRef(supabaseURL)
	if err != nil {
		return "", err
	}
	host = poolerHost(host)
	if host == "" {
		return "", errors.New("DB_POOLER_HOST es obligatoria (Dashboard → Connect)")
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.User(role + "." + ref),
		Host:     net.JoinHostPort(host, port),
		Path:     "/postgres",
		RawQuery: "sslmode=require",
	}
	return u.String(), nil
}

// projectRef toma el primer label del host de SUPABASE_URL (https://<ref>.supabase.co).
func projectRef(supabaseURL string) (string, error) {
	u, err := url.Parse(supabaseURL)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("SUPABASE_URL inválida")
	}
	ref, _, _ := strings.Cut(u.Hostname(), ".")
	if ref == "" {
		return "", fmt.Errorf("no se pudo obtener el project ref de SUPABASE_URL")
	}
	return ref, nil
}

// poolerHost acepta el host solo o la cadena completa copiada del Dashboard, y devuelve solo el host.
func poolerHost(v string) string {
	v = strings.TrimSpace(v)
	if _, after, found := strings.Cut(v, "@"); found {
		v = after
	}
	v = strings.TrimPrefix(v, "postgresql://")
	v = strings.TrimPrefix(v, "postgres://")
	if i := strings.IndexAny(v, ":/?"); i >= 0 {
		v = v[:i]
	}
	return v
}
