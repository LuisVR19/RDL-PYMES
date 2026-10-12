package certificate

import (
	"errors"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	ok := Info{KeyAlgorithm: "RSA", KeyBits: 2048, NotBefore: now.AddDate(0, -1, 0), NotAfter: now.AddDate(1, 0, 0)}
	cases := map[string]struct {
		mod  func(*Info)
		want error
	}{
		"RSA 2048 vigente":       {func(*Info) {}, nil},
		"RSA 4096":               {func(i *Info) { i.KeyBits = 4096 }, nil},
		"RSA 3072":               {func(i *Info) { i.KeyBits = 3072 }, ErrKeyNotAllowed},
		"ECDSA":                  {func(i *Info) { i.KeyAlgorithm, i.KeyBits = "ECDSA", 256 }, ErrKeyNotAllowed},
		"vencido":                {func(i *Info) { i.NotAfter = now.Add(-time.Second) }, ErrExpired},
		"vence en este instante": {func(i *Info) { i.NotAfter = now }, ErrExpired},
		"todavía no vigente":     {func(i *Info) { i.NotBefore = now.Add(time.Hour) }, ErrNotYetValid},
	}
	for name, c := range cases {
		i := ok
		c.mod(&i)
		if err := Validate(i, now); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: err = %v, se esperaba %v", name, err, c.want)
		}
	}
}
