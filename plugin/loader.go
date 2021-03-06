package plugin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"

	pm "plugin"
)

const pluginSymbolName = "NewPlugin"

type PluginsLoader struct {
	plugins []Symbol
	dir     string
	loaded  bool
}

func (pl *PluginsLoader) Plugins() []Symbol {
	return pl.plugins
}

func NewPluginsLoader(dir string) *PluginsLoader {

	pl := &PluginsLoader{
		dir: dir,
	}

	return pl
}

func (pl *PluginsLoader) LoadPlugins(force bool) error {

	if pl.loaded && !force {
		return nil
	}

	if _, err := os.Stat(pl.dir); err != nil {
		return err
	}

	plugins, err := listFiles(pl.dir, `*.so`)
	if err != nil {
		return err
	}

	for _, cmdPlugin := range plugins {
		pluginFile, err := pm.Open(path.Join(pl.dir, cmdPlugin.Name()))
		if err != nil {
			fmt.Printf("failed to open pluginFile %s: %v\n", cmdPlugin.Name(), err)
			continue
		}
		pluginSymbol, err := pluginFile.Lookup(pluginSymbolName)
		if err != nil {
			fmt.Printf("pluginFile %s does not export symbol \"%s\"\n",
				cmdPlugin.Name(), pluginSymbolName)
			continue
		}
		pl.plugins = append(pl.plugins, pluginSymbol.(Symbol))

	}

	pl.loaded = true

	return nil
}

func listFiles(dir, pattern string) ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filteredFiles := []os.FileInfo{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		matched, err := regexp.MatchString(pattern, file.Name())
		if err != nil {
			return nil, err
		}
		if matched {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles, nil
}
