package shall

import (
	"io/fs"
	"slices"
	"testing"
)

func TestSchemaHoldsEveryFileOfTheFormat(t *testing.T) {
	t.Attr("verifies", "R21")
	names, err := fs.Glob(Schema(), "*.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"chapter.schema.json", "items.schema.json", "manifest.schema.json", "mission.schema.json"}
	if !slices.Equal(names, want) {
		t.Errorf("schema files = %v, want %v", names, want)
	}
}
