package urlcache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStorePersistsByURLAndDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	url := "https://api.clickup.test/view/1"
	if changed, err := store.Put(url, []byte(`{"value":1}`)); err != nil || !changed {
		t.Fatalf("primer Put changed=%t err=%v", changed, err)
	}
	if changed, err := store.Put(url, []byte(`{"value":1}`)); err != nil || changed {
		t.Fatalf("Put igual changed=%t err=%v", changed, err)
	}
	if changed, err := store.Put(url, []byte(`{"value":2}`)); err != nil || !changed {
		t.Fatalf("Put nuevo changed=%t err=%v", changed, err)
	}

	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	body, hit, err := reopened.Get(url)
	if err != nil || !hit || string(body) != `{"value":2}` {
		t.Fatalf("Get body=%s hit=%t err=%v", body, hit, err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("archivos=%v", files)
	}
	info, err := os.Stat(files[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos=%v err=%v", info.Mode().Perm(), err)
	}
}

func TestStoreCaptureReplacesActiveURLs(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.Track("old")
	store.BeginCapture()
	store.Track("new-1")
	store.Track("new-2")
	store.CommitCapture()
	got := store.ActiveURLs()
	if len(got) != 2 {
		t.Fatalf("active=%v", got)
	}
	set := map[string]bool{got[0]: true, got[1]: true}
	if !reflect.DeepEqual(set, map[string]bool{"new-1": true, "new-2": true}) {
		t.Fatalf("active=%v", got)
	}
}

func TestStoreIgnoresCorruptEntry(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://api.clickup.test/task/1"
	if err := os.WriteFile(store.path(url), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := store.Get(url); err != nil || hit {
		t.Fatalf("hit=%t err=%v", hit, err)
	}
}
