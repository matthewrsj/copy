package towercontroller

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// _globalConfiguration is the global-level configuration file.
// Do not write to this outside of monitorConfig
var _globalConfiguration *Configuration

// MonitorConfig checks filepath for new changes and updates the globalConfiguration on changes
// nolint:gocognit // some complexity due to a bug in dependent library puts this over the top
func MonitorConfig(lg *zap.SugaredLogger, path string, initial *Configuration) {
	_globalConfiguration = initial

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		lg.Warnw("unable to create new watcher to monitor for configuration changes", "error", err.Error())
		return
	}

	defer func() {
		_ = watcher.Close()
	}()

	if err = watcher.Add(path); err != nil {
		lg.Warnw("unable to add file to monitor for configuration changes", "error", err.Error())
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				// routine shut down
				lg.Warnw("watcher to monitor for configuration changes shutting down")
				return
			}

			lg.Debugw("configuration file event detected", "event", event.String())

			// have to look for rename as well, as this seems to still be an issue
			// https://github.com/fsnotify/fsnotify/issues/92
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Remove == fsnotify.Remove {
				lg.Debug("configuration change detected")

				if conf, err := LoadConfig(path); err == nil {
					_globalConfiguration = &conf
				} else {
					lg.Warnw("unable to load configuration upon change", "error", err.Error())
				}

				if event.Op&fsnotify.Remove == fsnotify.Remove {
					_ = watcher.Remove(path)

					if err := watcher.Add(path); err != nil {
						lg.Warnw("unable to add file to monitor for configuration changes", "error", err.Error())
						return
					}
				}
			}
		case err, ok := <-watcher.Errors:
			lg.Warnw("error watching configuration file", "error", err.Error())

			if !ok {
				// routine shut down
				lg.Warnw("watcher to monitor for configuration changes shutting down")
				return
			}
		}
	}
}
