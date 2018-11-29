package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"

	"github.com/bitrise-tools/go-steputils/stepconf"
)

type config struct {
	Platform                string `env:"platform,opt[both,ios,android]"`
	IosAdditionalParams     string `env:"ios_additional_params"`
	AndroidAdditionalParams string `env:"android_additional_params"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func exportIosArtifacts() error {
	return nil
}

func exportAndroidArtifacts() error {
	return nil
}

func build(platform string, args ...string) error {
	buildCmd := command.New("flutter", append([]string{"build", platform}, args...)...).SetStdout(os.Stdout)

	fmt.Println()
	log.Donef("$ %s", buildCmd.PrintableCommandArgs())
	fmt.Println()

	return buildCmd.Run()
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)

	if cfg.Platform == "both" || cfg.Platform == "ios" {
		fmt.Println()
		log.Infof("Build iOS")

		if err := build("ios"); err != nil {
			failf("Failed to build iOS platform, error: %s", err)
		}

		if err := exportIosArtifacts(); err != nil {
			failf("Failed to export iOS artifacts, error: %s", err)
		}
	}

	if cfg.Platform == "both" || cfg.Platform == "android" {
		fmt.Println()
		log.Infof("Build Android")

		if err := build("apk"); err != nil {
			failf("Failed to build Android platform, error: %s", err)
		}

		if err := exportAndroidArtifacts(); err != nil {
			failf("Failed to export Android artifacts, error: %s", err)
		}
	}
}
