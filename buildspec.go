package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-tools/go-steputils/output"
	"github.com/bitrise-tools/go-steputils/tools"
	shellquote "github.com/kballard/go-shellquote"
	glob "github.com/ryanuber/go-glob"
)

type warning struct {
	s string
}

func (w warning) Error() string {
	return w.s
}

type buildSpecification struct {
	displayName          string
	platformCmdFlag      string
	platformSelectors    []string
	outputPathPattern    string
	additionalParameters string
}

func (spec buildSpecification) exportArtifacts(outputPathPattern string) error {
	location := os.Getenv("BITRISE_SOURCE_DIR")
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	switch spec.platformCmdFlag {
	case "apk":
		path, err := findPath(location, outputPathPattern, false)
		if err != nil {
			return err
		}

		deployedFilePath := filepath.Join(deployDir, filepath.Base(path))

		if err := output.ExportOutputFile(path, deployedFilePath, "BITRISE_APK_PATH"); err != nil {
			return err
		}
		log.Donef("- $BITRISE_APK_PATH: " + deployedFilePath)

		return nil
	case "ios":
		path, err := findPath(location, outputPathPattern, true)
		if err != nil {
			return err
		}

		fileName := filepath.Base(path)

		if err := ziputil.ZipDir(path, filepath.Join(deployDir, fileName+".zip"), false); err != nil {
			return err
		}
		log.Donef("- $BITRISE_DEPLOY_DIR/" + fileName + ".zip")

		if err := tools.ExportEnvironmentWithEnvman("BITRISE_APP_DIR_PATH", path); err != nil {
			return err
		}
		log.Donef("- $BITRISE_APP_DIR_PATH: " + path)

		return nil
	default:
		return warning{"unsupported platform for exporting artifacts"}
	}
}

func (spec buildSpecification) buildable(platform string) bool {
	return sliceutil.IsStringInSlice(platform, spec.platformSelectors)
}

func findPath(location string, outputPathPattern string, dir bool) (out string, err error) {
	err = filepath.Walk(location, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() == dir || !glob.Glob(outputPathPattern, path) {
			return nil
		}

		out = path
		return nil
	})
	if out == "" && err == nil {
		err = warning{"couldn't find output artifact on path: " + filepath.Join(location, outputPathPattern)}
	}
	return
}

func (spec buildSpecification) build(params string) error {
	paramSlice, err := shellquote.Split(params)
	if err != nil {
		return err
	}

	buildCmd := command.New("flutter", append([]string{"build", spec.platformCmdFlag},
		paramSlice...)...).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr)

	fmt.Println()
	log.Donef("$ %s", buildCmd.PrintableCommandArgs())
	fmt.Println()

	return buildCmd.Run()
}