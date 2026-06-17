package puller

import "testing"

func TestResourceName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Resource na raiz da pasta resources.
		{"rnld_api/server/main.lua", "rnld_api"},
		{"taskbar/fxmanifest.lua", "taskbar"},
		// Resource dentro de uma categoria entre colchetes.
		{"[gameplay]/rnld_api/client.lua", "rnld_api"},
		{"[local]/[hud]/rnld_api/ui/index.html", "rnld_api"},
		// Arquivo solto na raiz: não pertence a resource nenhum.
		{"server.cfg", ""},
		// Arquivo solto dentro de uma categoria: também não é um resource.
		{"[gameplay]/leiame.txt", ""},
		// Caminho vazio.
		{"", ""},
	}
	for _, c := range cases {
		if got := resourceName(c.path); got != c.want {
			t.Errorf("resourceName(%q) = %q, queria %q", c.path, got, c.want)
		}
	}
}
