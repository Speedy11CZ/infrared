package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Directory string `mapstructure:"directory"`
	Filename  string `mapstructure:"filename"`
	Watch     bool   `mapstructure:"watch"`
}

type File struct {
	config  FileConfig
	watcher *fsnotify.Watcher
	path    string
	isDir   bool
}

func NewFile(cfg FileConfig) (Provider, error) {
	path := cfg.Directory
	isDir := true
	if cfg.Filename != "" {
		if path != "" {
			return nil, errors.New("directory and filename are mutually exclusive")
		}
		path = cfg.Filename
		isDir = false
	}

	if path == "" {
		return nil, errors.New("directory or filename required")
	}

	return &File{
		config: cfg,
		path:   path,
		isDir:  isDir,
	}, nil
}

func (p *File) Provide(dataCh chan<- Data) (Data, error) {
	data, err := p.readConfigData()
	if err != nil {
		return Data{}, err
	}

	go func() {
		if err := p.watch(dataCh); err != nil {
			log.Error().
				Err(err).
				Str("provider", data.Type.String()).
				Msg("failed while watching provider")
		}
	}()

	return data, nil
}

// Configs returns a map with the relative file name as key and the config map as value
func (p File) Configs() map[string]map[string]any {
	cfgs := map[string]map[string]any{}
	for _, filePath := range p.filePaths() {
		cfg := map[string]any{}
		if err := ReadConfigFile(filePath, &cfg); err != nil {
			log.Error().
				Err(err).
				Str("path", filePath).
				Msg("failed to read config from file")
			continue
		}
		cfgs[filePath] = cfg
	}
	return cfgs
}

func (p *File) watch(dataCh chan<- Data) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	p.watcher = w

	if err := w.Add(p.path); err != nil {
		return err
	}

	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				log.Debug().
					Str("cause", "watcher event channel closed").
					Msg("closing file watcher")
				return nil
			}

			if e.Op&fsnotify.Remove == fsnotify.Remove ||
				e.Op&fsnotify.Write == fsnotify.Write ||
				e.Op&fsnotify.Create == fsnotify.Create ||
				e.Op&fsnotify.Rename == fsnotify.Rename ||
				e.Op == fsnotify.Remove {
				data, err := p.readConfigData()
				if err != nil {
					continue
				}
				dataCh <- data
			}
		case err, ok := <-w.Errors:
			if !ok {
				log.Debug().
					Str("cause", "watcher error channel closed").
					Msg("closing file watcher")
				return nil
			}

			log.Error().
				Err(err).
				Msg("error while watching directory")
		}
	}
}

func (p File) Close() error {
	if p.watcher != nil {
		return p.watcher.Close()
	}
	return nil
}

func (p File) filePaths() []string {
	if !p.isDir {
		return []string{p.path}
	}

	paths, err := filePathsFromDir(p.path)
	if err != nil {
		log.Error().
			Err(err).
			Str("directory", p.path).
			Msg("failed to read config from directory")
	}
	return paths
}

func (p File) readConfigData() (Data, error) {
	cfg := map[string]any{}
	for _, filePath := range p.filePaths() {
		fileCfg := map[string]any{}
		if err := ReadConfigFile(filePath, &fileCfg); err != nil {
			log.Error().
				Err(err).
				Str("path", filePath).
				Msg("failed to read config from file")
			continue
		}

		if err := mergo.Merge(&cfg, fileCfg, mergo.WithOverride); err != nil {
			log.Error().
				Err(err).
				Str("path", filePath).
				Msg("failed to merge configs")
		}
	}

	return Data{
		Type:   FileType,
		Config: cfg,
	}, nil
}

func filePathsFromDir(dir string) ([]string, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return nil, err
		}
	}

	filePaths := []string{}
	readConfig := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink == os.ModeSymlink {
			path, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}

			fi, err := os.Lstat(path)
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return nil
			}
		}

		filePaths = append(filePaths, path)
		return nil
	}

	return filePaths, filepath.WalkDir(dir, readConfig)
}

func ReadConfigFile(name string, v any) error {
	name, err := filepath.EvalSymlinks(name)
	if err != nil {
		return err
	}

	fi, err := os.Lstat(name)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		name, err = os.Readlink(name)
		if err != nil {
			return err
		}
	}

	bb, err := os.ReadFile(name)
	if err != nil {
		return err
	}

	ext := filepath.Ext(name)[1:]
	switch ext {
	case "json":
		if err := json.Unmarshal(bb, v); err != nil {
			return err
		}
	case "yml", "yaml":
		if err := yaml.Unmarshal(bb, v); err != nil {
			return err
		}
	default:
		return errors.New("unsupported file type")
	}
	return nil
}

func WriteConfigFile(path string, cfg map[string]any) error {
	dir, file := filepath.Split(path)
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return err
		}
	}
	path = filepath.Join(dir, file)

	var bb []byte
	switch ext := filepath.Ext(file)[1:]; ext {
	case "json":
		bb, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
	case "yml", "yaml":
		buf := bytes.NewBuffer([]byte{})
		enc := yaml.NewEncoder(buf)
		enc.SetIndent(2)
		if err := enc.Encode(cfg); err != nil {
			return err
		}
		enc.Close()
		bb = buf.Bytes()
	default:
		return errors.New("unsupported file type")
	}

	return os.WriteFile(path, bb, 0666)
}

func RemoveConfigFile(path string) error {
	dir, file := filepath.Split(path)
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		dir, err = os.Readlink(dir)
		if err != nil {
			return err
		}
	}
	path = filepath.Join(dir, file)
	return os.Remove(path)
}
