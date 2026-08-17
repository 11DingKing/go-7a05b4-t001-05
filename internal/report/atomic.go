package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"goldbar/internal/model"
)

// WriteAll builds the output files for the result and atomically commits them
// to dir. All files are staged to temp files first; only if every stage
// succeeds are they renamed into place. On failure no final file is modified,
// so a failed run never leaves a half-finished detail file. If the run has no
// errors, a stale errors.csv from a previous run is removed for consistency,
// making repeated runs idempotent.
func WriteAll(dir string, r *model.BatchResult) error {
	files := Build(r)
	if err := CommitFiles(dir, files); err != nil {
		return err
	}
	if _, ok := files[ErrorFile]; !ok {
		if err := os.Remove(filepath.Join(dir, ErrorFile)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理残留 %s 失败: %w", ErrorFile, err)
		}
	}
	return nil
}

// CommitFiles writes each file to a temp path in the same directory, fsyncs,
// then renames all to their final names. If staging any file fails, all staged
// temps are removed and no final file is touched.
func CommitFiles(dir string, files map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录 %q 失败: %w", dir, err)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	type staged struct{ tmp, final string }
	stagedFiles := make([]staged, 0, len(names))
	cleanup := func() {
		for _, sf := range stagedFiles {
			_ = os.Remove(sf.tmp)
		}
	}

	// Phase 1: stage every file to a temp path. No final file is touched yet.
	for _, name := range names {
		finalPath := filepath.Join(dir, name)
		tmpPath := filepath.Join(dir, "."+name+".swp")
		if err := writeSync(tmpPath, files[name]); err != nil {
			cleanup()
			return fmt.Errorf("暂存 %s 失败: %w", name, err)
		}
		stagedFiles = append(stagedFiles, staged{tmp: tmpPath, final: finalPath})
	}

	// Phase 2: atomically rename each staged file to its final name.
	for _, sf := range stagedFiles {
		if err := os.Rename(sf.tmp, sf.final); err != nil {
			for _, other := range stagedFiles {
				_ = os.Remove(other.tmp)
			}
			return fmt.Errorf("提交 %s 失败: %w", filepath.Base(sf.final), err)
		}
	}
	return nil
}

// writeSync creates or truncates path, writes data, fsyncs and closes.
func writeSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
