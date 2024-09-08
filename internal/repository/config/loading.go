package config

import (
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"sort"
	"sync"

	aulogging "github.com/StephanHCB/go-autumn-logging"
)

var (
	configurationData	*Application
	configurationLock	*sync.RWMutex
	configurationFilename	string
)

func init() {
	configurationData = &Application{}
	configurationLock = &sync.RWMutex{}

	flag.StringVar(&configurationFilename, "config", "", "config file path")
}

func ParseCommingLineFlags() {
	flag.Parse()
}

func parseAndOverwriteConfig(yamlFile []byte) error {
	newConfigurationData := &Application{}
	err := yaml.Unmarshal(yamlFile, newConfigurationData)
	if err != nil {
		aulogging.Logger.NoCtx().Error().Print("Failed to parse configuration: %v", err)
		return err
	}
	setConfigurationDefaults(newConfigurationData)
	applyEnvVarOverrides(newConfigurationData)

	/* validate all fields and log all errors */
	errs := url.Values{}
	validateServerConfiguration(errs, newConfigurationData.Server)
	validateDatabaseConfiguration(errs, newConfigurationData.Database)
	validateSerialPortConfiguration(errs, newConfigurationData.SerialPort)
	validateRatingConfiguration(errs, newConfigurationData.Rating)
	validateImageConfiguration(errs, newConfigurationData.Images)
	validateTimingConfiguration(errs, newConfigurationData.Timing)
	if len(errs) != 0 {
		var keys []string
		for key := range errs {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, k := range keys {
			key := k
			val := errs[k]
			aulogging.Logger.NoCtx().Error().Printf("configuration error: %s: %s", key, val[0])
		}
		return errors.New("configuration validation error")
	}

	/* safely exchange old config for new config */
	configurationLock.Lock()
	defer configurationLock.Unlock()
	configurationData = newConfigurationData
	return nil
}

func loadConfiguration() error {
	yamlFile, err := os.ReadFile(configurationFilename)
	if err != nil {
		aulogging.Logger.NoCtx().Error().Print("Failed to load configuration file '%s': %v", configurationFilename, err)
		return err
	}
	err = parseAndOverwriteConfig(yamlFile)
	return err
}

func StartupLoadConfiguration() error {
	aulogging.Logger.NoCtx().Info().Print("Reading configuration...")
	if configurationFilename == "" {
		aulogging.Logger.NoCtx().Error().Print("Configuration filename argument missig, use -config.")
		return errors.New("configuration file argument missing. Use -config to specify. Aborting.")
	}
	err := loadConfiguration()
	if err != nil {
		aulogging.Logger.NoCtx().Error().Print("Error reading or parsing config file: ", err)
		return errors.New(fmt.Sprintf("Error reading or parsing config file: %v", err))
	}
	return nil
}

func Configuration() *Application {
	configurationLock.RLock()
	defer configurationLock.RUnlock()
	return configurationData
}
