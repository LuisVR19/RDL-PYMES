// Package requestinfo guarda en el contexto datos del cliente que necesita la auditoría (IP y user-agent).
package requestinfo

import (
	"context"
	"net"
	"net/http"
	"net/netip"
)

type Info struct {
	IP        netip.Addr // inválida si no se pudo determinar
	UserAgent string
}

type ctxKey struct{}

func From(ctx context.Context) Info {
	info, _ := ctx.Value(ctxKey{}).(Info)
	return info
}

func With(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, ctxKey{}, info)
}

// Middleware toma la IP de la conexión. No confía en X-Forwarded-For: cualquiera puede enviarlo.
// TODO(infra): cuando se defina el balanceador, leer la IP del header que este fije, solo desde proxies de confianza.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := Info{UserAgent: truncate(r.UserAgent(), 512)}
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			if ip, err := netip.ParseAddr(host); err == nil {
				info.IP = ip.Unmap()
			}
		}
		next.ServeHTTP(w, r.WithContext(With(r.Context(), info)))
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
