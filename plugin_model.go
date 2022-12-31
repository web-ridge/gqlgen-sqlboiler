package gbgen

import (
	"fmt"
	"syscall"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/99designs/gqlgen/plugin/modelgen"
)

func NewModelPlugin() *ModelPlugin {
	return &ModelPlugin{}
}

type ModelPlugin struct {
}

func (m *ModelPlugin) GenerateCode(cfg *config.Config) (*codegen.Data, error) {
	_ = syscall.Unlink(cfg.Exec.Filename)
	if cfg.Model.IsDefined() {
		_ = syscall.Unlink(cfg.Model.Filename)
	}

	// LoadSchema again now we have everything
	if err := cfg.LoadSchema(); err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	if err := cfg.Init(); err != nil {
		return nil, fmt.Errorf("generating core failed: %w", err)
	}

	p := modelgen.New()
	if mut, ok := p.(plugin.ConfigMutator); ok {
		err := mut.MutateConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	// Merge again now that the generated structs have been injected into the typemap
	data, err := codegen.BuildData(cfg)
	if err != nil {
		return nil, fmt.Errorf("merging type systems failed: %w", err)
	}

	if err = codegen.GenerateCode(data); err != nil {
		return nil, fmt.Errorf("generating core failed: %w", err)
	}
	return data, nil
}
