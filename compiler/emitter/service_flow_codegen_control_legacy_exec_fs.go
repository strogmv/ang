package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepControlLegacyExecFS(st *flowRenderState, step normalizer.FlowStep, pad, sfx string, arg func(string) string) (string, bool) {
	switch step.Action {
	case "exec.Run":
		cmd := arg("cmd")
		output := arg("output")
		exitCodeVar := arg("exitCodeVar")
		failOnError := true
		if v, ok := step.Args["failOnError"].(bool); ok {
			failOnError = v
		}
		if cmd == "" {
			return "", true
		}
		var cmdArgs []string
		if v, ok := step.Args["args"]; ok {
			switch x := v.(type) {
			case []string:
				cmdArgs = x
			case string:
				if x != "" {
					cmdArgs = []string{x}
				}
			}
		}
		ecv, eov, eerv := "_execCmd"+sfx, "_execOut"+sfx, "_execErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := exec.CommandContext(ctx, %s", pad, ecv, cmd))
		for _, a := range cmdArgs {
			b.WriteString(fmt.Sprintf(", %s", a))
		}
		b.WriteString(")\n")
		if stdin := arg("stdin"); stdin != "" {
			b.WriteString(fmt.Sprintf("%s%s.Stdin = strings.NewReader(%s)\n", pad, ecv, stdin))
		}
		b.WriteString(fmt.Sprintf("%s%s, %s := %s.CombinedOutput()\n", pad, eov, eerv, ecv))
		if exitCodeVar != "" {
			assign := ":="
			if st.declared[exitCodeVar] {
				assign = "="
			}
			st.declared[exitCodeVar] = true
			st.pointers[exitCodeVar] = false
			b.WriteString(fmt.Sprintf("%s%s %s 0\n", pad, exitCodeVar, assign))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n%s\tif _ee, _ok := %s.(*exec.ExitError); _ok { %s = _ee.ExitCode() }\n%s}\n", pad, eerv, pad, eerv, exitCodeVar, pad))
		}
		if failOnError {
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, eerv))
			b.WriteString(fmt.Sprintf("%s\t"+`return resp, fmt.Errorf("exec: %%s: %%w", string(%s), %s)`+"\n", pad, eov, eerv))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, eov))
		}
		return b.String(), true

	case "fs.TempDir":
		output := arg("output")
		if output == "" {
			return "", true
		}
		pattern := arg("pattern")
		if pattern == "" {
			pattern = `"ang-tmp-*"`
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		tdv, tdev := "_tmpDir"+sfx, "_tmpDirErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := os.MkdirTemp(\"\", %s)\n", pad, tdv, tdev, pattern))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, tdev))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"temp dir: %w\", "+tdev+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, tdv))
		return b.String(), true

	case "fs.WriteFile":
		path := arg("path")
		data := arg("data")
		if path == "" || data == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _mkErr := os.MkdirAll(filepath.Dir(%s), 0o755); _mkErr != nil {\n", pad, path))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"mkdir: %w\", _mkErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _wErr := os.WriteFile(%s, []byte(%s), 0o644); _wErr != nil {\n", pad, path, data))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"write file: %w\", _wErr)"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "fs.ReadFile":
		path := arg("path")
		output := arg("output")
		if path == "" || output == "" {
			return "", true
		}
		optional := false
		if v, ok := step.Args["optional"].(bool); ok {
			optional = v
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		rfbv, rferrv := "_rfBytes"+sfx, "_rfErr"+sfx
		var b strings.Builder
		if optional {
			b.WriteString(fmt.Sprintf("%s%s, %s := os.ReadFile(%s)\n", pad, rfbv, rferrv, path))
			if assign == ":=" {
				b.WriteString(fmt.Sprintf("%s%s := \"\"\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%sif %s == nil { %s = string(%s) }\n", pad, rferrv, output, rfbv))
		} else {
			b.WriteString(fmt.Sprintf("%s%s, %s := os.ReadFile(%s)\n", pad, rfbv, rferrv, path))
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rferrv))
			b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"read file: %w\", "+rferrv+")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			b.WriteString(fmt.Sprintf("%s%s %s string(%s)\n", pad, output, assign, rfbv))
		}
		return b.String(), true

	case "fs.Remove":
		path := arg("path")
		if path == "" {
			return "", true
		}
		return fmt.Sprintf("%sdefer os.RemoveAll(%s)\n", pad, path), true

	case "archive.ZipDir":
		// archive.ZipDir: zip a local directory tree into []byte.
		// Args: path (dir to zip), output (var name for []byte result).
		srcPath := arg("path")
		output := arg("output")
		if srcPath == "" || output == "" {
			return "", true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		// unique temp var names
		bufv := "_zipBuf" + sfx
		wv := "_zipW" + sfx
		walkErrv := "_zipWalkErr" + sfx
		closeErrv := "_zipCloseErr" + sfx
		pathv := "_zPath" + sfx
		infov := "_zInfo" + sfx
		errv := "_zErr" + sfx
		relv := "_zRel" + sfx
		fwv := "_zFW" + sfx
		fv := "_zF" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := &bytes.Buffer{}\n", pad, bufv))
		b.WriteString(fmt.Sprintf("%s%s := zip.NewWriter(%s)\n", pad, wv, bufv))
		b.WriteString(fmt.Sprintf("%s%s := filepath.Walk(%s, func(%s string, %s os.FileInfo, %s error) error {\n", pad, walkErrv, srcPath, pathv, infov, errv))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil { return %s }\n", pad, errv, errv))
		b.WriteString(fmt.Sprintf("%s\tif %s.IsDir() { return nil }\n", pad, infov))
		b.WriteString(fmt.Sprintf("%s\t%s, _ := filepath.Rel(%s, %s)\n", pad, relv, srcPath, pathv))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := %s.Create(%s)\n", pad, fwv, errv, wv, relv))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil { return %s }\n", pad, errv, errv))
		b.WriteString(fmt.Sprintf("%s\t%s, %s := os.Open(%s)\n", pad, fv, errv, pathv))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil { return %s }\n", pad, errv, errv))
		b.WriteString(fmt.Sprintf("%s\tdefer %s.Close()\n", pad, fv))
		b.WriteString(fmt.Sprintf("%s\t_, %s = io.Copy(%s, %s)\n", pad, errv, fwv, fv))
		b.WriteString(fmt.Sprintf("%s\treturn %s\n", pad, errv))
		b.WriteString(fmt.Sprintf("%s})\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, walkErrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"zip walk: %w\", "+walkErrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s := %s.Close(); %s != nil {\n", pad, closeErrv, wv, closeErrv))
		b.WriteString(errReturn(st, pad+"\t", "fmt.Errorf(\"zip close: %w\", "+closeErrv+")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s %s.Bytes()\n", pad, output, assign, bufv))
		return b.String(), true

	case "session.Get":
		// session.Get: read anonymous session ID from request context.
		// Args: output (var name for string result).
		output := arg("output")
		if output == "" {
			return "", true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		return fmt.Sprintf("%s%s %s reqctx.SessionID(ctx)\n", pad, output, assign), true
	}

	return "", false
}
