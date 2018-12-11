package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

type config struct {
	IOSAdditionalParams     string `env:"ios_additional_params"`
	AndroidAdditionalParams string `env:"android_additional_params"`
	Platform                string `env:"platform,opt[both,ios,android]"`
	IOSExportPattern        string `env:"ios_output_pattern"`
	AndroidExportPattern    string `env:"android_output_pattern"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)

	for _, spec := range []buildSpecification{
		buildSpecification{
			displayName:          "iOS",
			platformCmdFlag:      "ios",
			platformSelectors:    []string{"both", "ios"},
			outputPathPattern:    cfg.IOSExportPattern,
			additionalParameters: cfg.IOSAdditionalParams,
		},
		buildSpecification{
			displayName:          "Android",
			platformCmdFlag:      "apk",
			platformSelectors:    []string{"both", "android"},
			outputPathPattern:    cfg.AndroidExportPattern,
			additionalParameters: cfg.AndroidAdditionalParams,
		},
	} {
		if !spec.buildable(cfg.Platform) {
			continue
		}

		fmt.Println()
		log.Infof("Build " + spec.displayName)
		if err := spec.build(spec.additionalParameters); err != nil {
			failf("Failed to build %s platform, error: %s", spec.displayName, err)
		}

		fmt.Println()
		log.Infof("Export " + spec.displayName + " artifact")

		if err := spec.exportArtifacts(spec.outputPathPattern); err != nil {
			failf("Failed to export %s artifacts, error: %s", spec.displayName, err)
		}
	}
}
