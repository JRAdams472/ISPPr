package isppr

import "testing"

func TestAssetsEmbed(t *testing.T) {
	for _, name := range []string{"migrations/0001_init.up.sql", "seed/001_demo.sql"} {
		if _, err := Assets.ReadFile(name); err != nil {
			t.Fatalf("missing embedded asset %s: %v", name, err)
		}
	}
}
