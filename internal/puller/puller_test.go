package puller

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResourceName(t *testing.T) {
	// Monta uma árvore parecida com a de um servidor FiveM, com o ".git" na raiz
	// do servidor e os resources dentro de "resources" (alguns em categorias
	// entre colchetes), cada um com seu fxmanifest.lua.
	repo := t.TempDir()
	for _, dir := range []string{
		"resources/vrp",
		"resources/[scripts]/grime",
		"resources/[maps]/cfx-gabz",
		"resources/[scripts]/[sub]/aninhado",
	} {
		full := filepath.Join(repo, filepath.FromSlash(dir))
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "fxmanifest.lua"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		path string
		want string
	}{
		// Resource direto em resources/.
		{"resources/vrp/fxmanifest.lua", "vrp"},
		{"resources/vrp/server/main.lua", "vrp"},
		// Resource dentro de uma categoria.
		{"resources/[scripts]/grime/client.lua", "grime"},
		{"resources/[maps]/cfx-gabz/stream/a.ydr", "cfx-gabz"},
		// Resource dentro de categorias aninhadas.
		{"resources/[scripts]/[sub]/aninhado/x.lua", "aninhado"},
		// Arquivos fora de qualquer resource.
		{"server.cfg", ""},
		{"resources/server.cfg", ""},
		{"resources/[scripts]/leiame.txt", ""},
		// Caminho de um resource inexistente no disco (ex.: removido no pull).
		{"resources/[scripts]/fantasma/main.lua", ""},
		// Caminho vazio.
		{"", ""},
	}
	for _, c := range cases {
		if got := resourceName(repo, c.path); got != c.want {
			t.Errorf("resourceName(%q) = %q, queria %q", c.path, got, c.want)
		}
	}
}
