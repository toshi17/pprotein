package slp

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/kaz/pprotein/internal/collect"
	"github.com/kaz/pprotein/internal/extproc"
	"github.com/kaz/pprotein/internal/persistent"
	"github.com/kaz/pprotein/internal/storage"
	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
)

type (
	handler struct {
		opts   *collect.Options
		config *persistent.Handler
		db     string
	}
)

//go:embed slp.yml
var defaultConfig []byte

func NewHandler(opts *collect.Options, store storage.Storage) (*handler, error) {
	h := &handler{
		opts: opts,
		db:   os.Getenv("PPROTEIN_SLOWLOG_DB"),
	}
	if h.db != "" && h.db != "mysql" && h.db != "postgresql" {
		return nil, fmt.Errorf("PPROTEIN_SLOWLOG_DB must be mysql or postgresql: %q", h.db)
	}

	config, err := persistent.New(store, "slp.yml", defaultConfig, h.sanitize)
	if err != nil {
		return nil, fmt.Errorf("failed to create targets: %w", err)
	}
	h.config = config

	return h, nil
}

func (h *handler) Register(g *echo.Group) error {
	h.config.RegisterHandlers(g.Group("/config"))

	if err := extproc.NewHandler(&processor{confPath: h.config.GetPath(), db: h.db}, h.opts).Register(g); err != nil {
		return fmt.Errorf("failed to register extproc handlers: %w", err)
	}
	return nil
}

func (h *handler) sanitize(raw []byte) ([]byte, error) {
	var config interface{}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	res, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	return res, nil
}
