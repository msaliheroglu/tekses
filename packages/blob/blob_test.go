package blob

import (
	"context"
	"errors"
	"testing"
)

func TestFSPutGet(t *testing.T) {
	fs, err := NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := fs.Put(ctx, "abc.json", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	// İdempotent: aynı anahtara ikinci yazım hatasızdır ve içerik korunur.
	if err := fs.Put(ctx, "abc.json", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	data, err := fs.Get(ctx, "abc.json")
	if err != nil || string(data) != `{"a":1}` {
		t.Fatalf("Get = %q, %v", data, err)
	}

	if _, err := fs.Get(ctx, "yok.json"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("olmayan anahtar = %v, beklenen ErrNotFound", err)
	}
}

func TestFSRejectsPathEscape(t *testing.T) {
	fs, err := NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../kacis", "a/b", `a\b`, "", "..", "x..y/../z"} {
		if err := fs.Put(context.Background(), key, []byte("x")); err == nil {
			t.Errorf("anahtar %q kabul edildi, reddedilmeliydi", key)
		}
	}
}
