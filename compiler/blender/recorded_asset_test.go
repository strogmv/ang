package blender

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRecordedAssetRebindsRelocatedProvenanceToCurrentContract(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.py")
	if err = os.WriteFile(path, []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import runpy,sys,pathlib,hashlib
root=pathlib.Path(sys.argv[1]);resolve=runpy.run_path(str(root/'runtime.py'))['recorded_asset']
asset=root/'current.glb';asset.write_bytes(b'existing authored asset')
row={'source_asset':str(root/'deleted-stage'/'source.glb'),'source_sha256':hashlib.sha256(asset.read_bytes()).hexdigest()}
assert resolve(row,{'source_asset':str(asset)})==asset
# A different current contract asset may never fall back to an old, still
# existing file with the expected hash.
old=root/'old.glb';old.write_bytes(asset.read_bytes());row['source_asset']=str(old)
asset.write_bytes(b'changed source')
assert resolve(row,{'source_asset':str(asset)}) is None
assert resolve(row,{'source_asset':str(root/'missing.glb')}) is None
assert resolve(row,{})==old
assert resolve({}, {'source_asset':str(old)}) is None
`
	output, err := exec.Command(python, "-B", "-c", script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
