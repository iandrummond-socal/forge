package site

import (
	"fmt"
	"github.com/BurntSushi/toml"
)

// decodeTOML keeps the schema strict except for explicitly named open tables.
// Callers must validate the type of an open table before using its contents.
// Front matter callers prefix a newline so parser line numbers match the file.
func decodeTOML(name, source string, target any, openTables ...string) error {
	metadata, err := toml.Decode(source, target)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	for _, key := range metadata.Undecoded() {
		allowed := false
		for _, table := range openTables {
			if len(key) > 0 && key[0] == table {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%s: unknown field %s", name, key)
		}
	}
	return nil
}
