package counter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "llmctl", "counter.json")
}

func Load() (State, error) {
	var s State
	b, err := os.ReadFile(path())
	if err != nil {
		return s, nil // leer = ok
	}
	_ = json.Unmarshal(b, &s)
	return s, nil
}

func Save(s State) error {
	p := path()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0o644)
}
