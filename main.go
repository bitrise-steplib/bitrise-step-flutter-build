package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	shellquote "github.com/kballard/go-shellquote"
)

type config struct {
	Platform                string `env:"platform,opt[both,ios,android]"`
	IOSAdditionalParams     string `env:"ios_additional_params"`
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

func build(platform string, params string) (*command.Model, error) {
	paramSlice, err := shellquote.Split(params)
	if err != nil {
		return nil, err
	}

	buildArgs := []string{"build", platform}
	return command.New("flutter", append(buildArgs, paramSlice...)...).SetStdout(os.Stdout).SetStderr(os.Stderr), nil
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

		iOSBuildCmd, err := build("ios", cfg.IOSAdditionalParams)
		if err != nil {
			failf("Failed to generate iOS build command, error: %s", err)
		}

		fmt.Println()
		log.Donef("$ %s", iOSBuildCmd.PrintableCommandArgs())
		fmt.Println()

		if err := iOSBuildCmd.Run(); err != nil {
			failf("Failed to build iOS platform, error: %s", err)
		}

		if err := exportIosArtifacts(); err != nil {
			failf("Failed to export iOS artifacts, error: %s", err)
		}
	}

	if cfg.Platform == "both" || cfg.Platform == "android" {
		fmt.Println()
		log.Infof("Build Android")

		androidBuildCmd, err := build("apk", cfg.AndroidAdditionalParams)
		if err != nil {
			failf("Failed to generate Android build command, error: %s", err)
		}

		fmt.Println()
		log.Donef("$ %s", androidBuildCmd.PrintableCommandArgs())
		fmt.Println()

		if err := androidBuildCmd.Run(); err != nil {
			failf("Failed to build Android platform, error: %s", err)
		}

		if err := exportAndroidArtifacts(); err != nil {
			failf("Failed to export Android artifacts, error: %s", err)
		}
	}
}
