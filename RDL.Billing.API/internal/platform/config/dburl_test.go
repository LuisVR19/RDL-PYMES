package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPoolerURLAcceptsHostOrFullConnectionString(t *testing.T) {
	want := "postgres://billing_api.abcref@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require"
	for _, host := range []string{
		"aws-0-us-east-1.pooler.supabase.com",
		"  aws-0-us-east-1.pooler.supabase.com  ",
		"postgresql://postgres.abcref:[YOUR-PASSWORD]@aws-0-us-east-1.pooler.supabase.com:5432/postgres",
	} {
		got, err := PoolerURL("https://abcref.supabase.co", host, "billing_api", PoolerTransactionPort)
		if err != nil || got != want {
			t.Errorf("host %q: got %q err %v", host, got, err)
		}
	}
}

func TestPoolerURLRequiresHost(t *testing.T) {
	if _, err := PoolerURL("https://abcref.supabase.co", "", "billing_api", PoolerTransactionPort); err == nil {
		t.Fatal("se esperaba error sin host")
	}
}

func TestLoadBuildsDatabaseURLFromPoolerHost(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"SUPABASE_URL":   "https://abcref.supabase.co",
		"DB_POOLER_HOST": "aws-0-us-east-1.pooler.supabase.com",
		"DB_PASSWORD":    `p@ss"#&word`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.URL != "postgres://billing_api.abcref@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require" {
		t.Fatalf("url=%q", cfg.DB.URL)
	}
	if cfg.DB.Password != `p@ss"#&word` {
		t.Fatal("la contraseña debe pasar intacta, fuera de la URL")
	}
}

func TestLoadDotEnvParsesQuotesAndKeepsExistingVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "\xef\xbb\xbf# comentario\n" +
		"DOTENV_T_A=simple # comentario\n" +
		"DOTENV_T_B='p@ss\"#&$word'\n" +
		"DOTENV_T_C=\"con espacios\"\n" +
		"export DOTENV_T_D=exportada\n" +
		"DOTENV_T_E=del archivo\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_T_E", "del entorno")
	for _, k := range []string{"DOTENV_T_A", "DOTENV_T_B", "DOTENV_T_C", "DOTENV_T_D"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"DOTENV_T_A": "simple",
		"DOTENV_T_B": `p@ss"#&$word`,
		"DOTENV_T_C": "con espacios",
		"DOTENV_T_D": "exportada",
		"DOTENV_T_E": "del entorno",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s=%q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnvMissingFileIsFine(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "no-existe")); err != nil {
		t.Fatal(err)
	}
}
