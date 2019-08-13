package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/newrelic/nri-flex/internal/config"
	"github.com/newrelic/nri-flex/internal/discovery"
	"github.com/newrelic/nri-flex/internal/load"
	"github.com/newrelic/nri-flex/internal/logger"
	"github.com/newrelic/nri-flex/internal/outputs"
	"github.com/sirupsen/logrus"
)

// RunFlex runs flex
// if mode is "" run in default mode
func RunFlex(mode string) {
	logCheck()

	logger.Flex("debug", nil, fmt.Sprintf("%v: v%v %v:%v", load.IntegrationName, load.IntegrationVersion, runtime.GOOS, runtime.GOARCH), false)

	// store config ymls
	configs := []load.Config{}

	switch mode {
	case "lambda":
		addConfigsFromPath("/var/task/pkg/flexConfigs/", &configs)
		if config.SyncGitConfigs("/tmp/") {
			addConfigsFromPath("/tmp/", &configs)
		}
	default:
		// running as default
		config.SyncGitConfigs("")
		if load.Args.ConfigFile != "" {
			addSingleConfigFile(load.Args.ConfigFile, &configs)
		} else {
			addConfigsFromPath(load.Args.ConfigDir, &configs)
		}
		if load.Args.ContainerDiscovery || load.Args.Fargate {
			discovery.Run(&configs)
		}
	}

	if load.ContainerID == "" && mode != "test" && mode != "lambda" {
		discovery.Processes()
	}

	logger.Flex("debug", nil, fmt.Sprintf("config files loaded %d", len(configs)), false)
	config.RunFiles(&configs)
	outputs.StatusSample()

	if load.Args.InsightsURL != "" && load.Args.InsightsAPIKey != "" {
		outputs.SendToInsights()
	} else if load.Args.MetricAPIUrl != "" && (load.Args.InsightsAPIKey != "" || load.Args.MetricAPIKey != "") && len(load.MetricsStore.Data) > 0 {
		outputs.SendToMetricAPI()
	} else if len(load.MetricsStore.Data) > 0 && (load.Args.MetricAPIUrl == "" || (load.Args.InsightsAPIKey == "" || load.Args.MetricAPIKey == "")) {
		logger.Flex("debug", nil, "metric_api is being used, but metric url and/or key has not been set", false)
	}
}

func addSingleConfigFile(configFile string, configs *[]load.Config) {
	file, err := os.Stat(configFile)
	logger.Flex("fatal", err, "failed to read specified config file: "+configFile, false)
	path := strings.Replace(filepath.FromSlash(configFile), file.Name(), "", -1)
	files := []os.FileInfo{file}
	config.LoadFiles(configs, files, path)
}

func addConfigsFromPath(path string, configs *[]load.Config) {
	configPath := filepath.FromSlash(path)
	files, err := ioutil.ReadDir(configPath)
	logger.Flex("fatal", err, fmt.Sprintf("failed to read config dir %v", path), false)
	config.LoadFiles(configs, files, configPath)
}

// SetDefaults set flex defaults
func SetDefaults() {
	load.Logrus.Out = os.Stdout
	load.FlexStatusCounter.M = make(map[string]int)
	load.FlexStatusCounter.M["EventCount"] = 0
	load.FlexStatusCounter.M["EventDropCount"] = 0
	load.FlexStatusCounter.M["ConfigsProcessed"] = 0
}

// SetEnvs set environment variable argument overrides
func SetEnvs() {
	load.AWSExecutionEnv = os.Getenv("AWS_EXECUTION_ENV")
	gitService := os.Getenv("GIT_SERVICE")
	if gitService != "" {
		load.Args.GitService = gitService
	}
	gitRepo := os.Getenv("GIT_REPO")
	if gitRepo != "" {
		load.Args.GitRepo = gitRepo
		load.Args.GitToken = os.Getenv("GIT_TOKEN")
		load.Args.GitUser = os.Getenv("GIT_USER")
	}
	insightsAPIKey := os.Getenv("INSIGHTS_API_KEY")
	if insightsAPIKey != "" {
		load.Args.InsightsAPIKey = insightsAPIKey
		load.Args.InsightsURL = os.Getenv("INSIGHTS_URL")
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		load.IsKubernetes = true
	}
	eventLimit, err := strconv.Atoi(os.Getenv("EVENT_LIMIT"))
	if err == nil && eventLimit > 0 {
		load.Args.EventLimit = eventLimit
	}
	configSync, err := strconv.ParseBool(os.Getenv("PROCESS_CONFIGS_SYNC"))
	if err == nil && configSync {
		load.Args.ProcessConfigsSync = configSync
	}
	fargate, err := strconv.ParseBool(os.Getenv("FARGATE"))
	if err == nil && fargate {
		load.Args.Fargate = fargate
	}
	cd, err := strconv.ParseBool(os.Getenv("CONTAINER_DISCOVERY"))
	if err == nil && cd {
		load.Args.ContainerDiscovery = cd
	}
	load.Args.MetricAPIUrl = os.Getenv("METRIC_API_URL")
	load.Args.MetricAPIKey = os.Getenv("METRIC_API_KEY")
}

func logCheck() {
	if load.Args.Verbose && os.Getenv("VERBOSE") != "true" && os.Getenv("VERBOSE") != "1" && load.AWSExecutionEnv == "" {
		load.Logrus.SetLevel(logrus.TraceLevel) // do not do verbose logging for infra agent
	} else if os.Getenv("VERBOSE") == "true" && load.AWSExecutionEnv != "" {
		load.Logrus.SetLevel(logrus.TraceLevel)
	}
}
