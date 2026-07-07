package paymentprovider

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/strogmv/ang/compiler"
)

// Load reads provider intent from <project>/<cueRoot>/provider.cue.
// When schemaDir is non-empty, shared schema files are loaded from that directory
// instead of <project>/<cueRoot>/schema/ (monorepo layout: schema_dir in ang.yaml).
func Load(projectPath, cueRoot, schemaDir string) (*ProviderSpec, error) {
	if cueRoot == "" {
		cueRoot = compiler.DefaultCueRoot
	}
	cueDir := filepath.Join(projectPath, cueRoot)

	overlay, err := schemaOverlay(cueDir, schemaDir)
	if err != nil {
		return nil, err
	}
	cfg := &load.Config{Dir: cueDir}
	if len(overlay) > 0 {
		cfg.Overlay = overlay
	}

	insts := load.Instances([]string{"."}, cfg)
	if len(insts) == 0 {
		return nil, fmt.Errorf("no cue instances in %s", cueDir)
	}
	if err := firstLoadError(insts); err != nil {
		return nil, fmt.Errorf("load cue from %s: %w", cueDir, err)
	}

	ctx := cuecontext.New()
	val := ctx.BuildInstance(insts[0])
	if err := val.Err(); err != nil {
		return nil, fmt.Errorf("build cue instance: %w", err)
	}

	provider := val.LookupPath(cue.ParsePath("provider"))
	if !provider.Exists() {
		return nil, fmt.Errorf("missing top-level \"provider\" in %s", cueDir)
	}
	if err := provider.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("validate provider: %w", err)
	}

	var spec ProviderSpec
	if err := provider.Decode(&spec); err != nil {
		return nil, fmt.Errorf("decode provider: %w", err)
	}
	if spec.PackageName == "" {
		return nil, fmt.Errorf("provider.package_name is required")
	}
	if spec.SID == "" {
		return nil, fmt.Errorf("provider.sid is required")
	}
	if spec.StructName == "" {
		return nil, fmt.Errorf("provider.struct_name is required")
	}
	if spec.ConstructorName == "" {
		spec.ConstructorName = "New"
	}
	if spec.PaymentSource == "" {
		spec.PaymentSource = "apm"
	}
	if spec.Secrets.Separator == "" {
		spec.Secrets.Separator = ":"
	}
	if strings.EqualFold(spec.APICompat, "macan_p2p") && spec.HasPayin {
		return nil, fmt.Errorf("macan_p2p: has_payin must be false (use has_p2p + InitPayP2P)")
	}
	return &spec, nil
}

func firstLoadError(insts []*build.Instance) error {
	for _, inst := range insts {
		if inst.Err != nil {
			return inst.Err
		}
	}
	return nil
}
