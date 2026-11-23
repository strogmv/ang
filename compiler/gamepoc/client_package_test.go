package gamepoc

import "testing"

func TestClientPackageIdentifiesEmittedViews(t *testing.T) {
	id := clientPackageID("world", "wire", []byte("art"), []byte("floor"))
	for _, altered := range []string{
		clientPackageID("world2", "wire", []byte("art"), []byte("floor")),
		clientPackageID("world", "wire2", []byte("art"), []byte("floor")),
		clientPackageID("world", "wire", []byte("art2"), []byte("floor")),
		clientPackageID("world", "wire", []byte("art"), []byte("floor2")),
	} {
		if altered == id {
			t.Fatal("different world, contract or scene view accepted the same handshake")
		}
	}
}
